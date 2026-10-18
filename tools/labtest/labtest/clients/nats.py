"""NATS messaging client for Kootenai platform."""

from __future__ import annotations

import asyncio
import json
import logging
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Callable

import nats
from nats.js import JetStreamContext

logger = logging.getLogger(__name__)


@dataclass
class ReceivedMessage:
    """A message received from NATS."""

    subject: str
    data: dict[str, Any]
    received_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))


class NATSClient:
    """Async NATS client for messaging tests.

    Usage:
        async with NATSClient("nats://localhost:4222").session() as nats:
            await nats.publish("labs.events", {"type": "test"})
    """

    def __init__(self, url: str):
        """Initialize NATS client.

        Args:
            url: NATS server URL (e.g., nats://localhost:4222)
        """
        self.url = url
        self._nc: nats.NATS | None = None
        self._js: JetStreamContext | None = None
        self._subscriptions: list = []
        self._received_messages: list[ReceivedMessage] = []

    @asynccontextmanager
    async def session(self):
        """Context manager for NATS connection."""
        self._nc = await nats.connect(self.url)
        self._js = self._nc.jetstream()
        self._received_messages = []
        try:
            yield self
        finally:
            # Drain subscriptions
            for sub in self._subscriptions:
                try:
                    await sub.unsubscribe()
                except Exception:
                    pass
            self._subscriptions = []

            if self._nc:
                await self._nc.close()
                self._nc = None
                self._js = None

    async def is_connected(self) -> bool:
        """Check if connected to NATS.

        Returns:
            True if connected
        """
        return self._nc is not None and self._nc.is_connected

    async def ping(self) -> bool:
        """Test NATS connectivity with a round-trip.

        Returns:
            True if ping successful
        """
        if not self._nc:
            return False

        try:
            await self._nc.flush(timeout=5)
            return True
        except Exception:
            return False

    # JetStream operations

    async def get_stream_info(self, stream_name: str = "LABS") -> dict[str, Any] | None:
        """Get JetStream stream information.

        Args:
            stream_name: Name of the stream

        Returns:
            Stream info dict or None if not found
        """
        try:
            info = await self._js.stream_info(stream_name)
            return {
                "name": info.config.name,
                "subjects": info.config.subjects,
                "messages": info.state.messages,
                "bytes": info.state.bytes,
                "first_seq": info.state.first_seq,
                "last_seq": info.state.last_seq,
            }
        except Exception as e:
            logger.warning(f"Failed to get stream info: {e}")
            return None

    async def publish(self, subject: str, data: dict[str, Any]) -> bool:
        """Publish a message to JetStream.

        Args:
            subject: Subject to publish to
            data: Message payload (will be JSON encoded)

        Returns:
            True if published successfully
        """
        try:
            payload = json.dumps(data).encode()
            await self._js.publish(subject, payload)
            return True
        except Exception as e:
            logger.error(f"Failed to publish to {subject}: {e}")
            return False

    async def subscribe(
        self,
        subject: str,
        callback: Callable[[str, dict], None] | None = None,
        queue: str | None = None,
    ) -> None:
        """Subscribe to a subject.

        Args:
            subject: Subject pattern to subscribe to
            callback: Optional callback for received messages
            queue: Optional queue group name
        """

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
            except json.JSONDecodeError:
                data = {"raw": msg.data.decode()}

            received = ReceivedMessage(subject=msg.subject, data=data)
            self._received_messages.append(received)

            if callback:
                callback(msg.subject, data)

        if queue:
            sub = await self._nc.subscribe(subject, queue=queue, cb=message_handler)
        else:
            sub = await self._nc.subscribe(subject, cb=message_handler)

        self._subscriptions.append(sub)

    async def wait_for_message(
        self,
        subject_pattern: str | None = None,
        message_type: str | None = None,
        timeout: float = 30.0,
    ) -> ReceivedMessage | None:
        """Wait for a message matching criteria.

        Args:
            subject_pattern: Subject must contain this string
            message_type: Message data must have this "type" field
            timeout: Maximum wait time in seconds

        Returns:
            Matching message or None if timeout
        """
        start = asyncio.get_running_loop().time()

        while asyncio.get_running_loop().time() - start < timeout:
            for msg in self._received_messages:
                # Check subject pattern
                if subject_pattern and subject_pattern not in msg.subject:
                    continue

                # Check message type
                if message_type and msg.data.get("type") != message_type:
                    continue

                return msg

            await asyncio.sleep(0.1)

        return None

    def get_received_messages(self) -> list[ReceivedMessage]:
        """Get all received messages.

        Returns:
            List of received messages
        """
        return self._received_messages.copy()

    def clear_messages(self) -> None:
        """Clear received messages buffer."""
        self._received_messages = []

    # Convenience methods for Kootenai events

    async def publish_vm_event(
        self,
        pod_id: str,
        vm_name: str,
        event_type: str,
        data: dict[str, Any] | None = None,
    ) -> bool:
        """Publish a VM event.

        Args:
            pod_id: Pod ID
            vm_name: VM name
            event_type: Event type (e.g., "file.created", "process.started")
            data: Additional event data

        Returns:
            True if published
        """
        event = {
            "type": event_type,
            "podId": pod_id,
            "vmName": vm_name,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            **(data or {}),
        }
        return await self.publish(f"labs.events.{pod_id}", event)

    async def subscribe_to_pod_events(self, pod_id: str) -> None:
        """Subscribe to events for a specific pod.

        Args:
            pod_id: Pod ID to subscribe to
        """
        await self.subscribe(f"labs.events.{pod_id}")

    async def subscribe_to_all_events(self) -> None:
        """Subscribe to all lab events."""
        await self.subscribe("labs.events.>")

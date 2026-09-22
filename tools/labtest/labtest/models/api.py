"""Pydantic models for API responses."""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel, ConfigDict, Field


class PodStatus(str, Enum):
    """Pod lifecycle status."""

    PROVISIONING = "provisioning"
    RUNNING = "running"
    STOPPED = "stopped"
    ERROR = "error"
    DESTROYING = "destroying"
    DESTROYED = "destroyed"


class CheckpointStatus(str, Enum):
    """Checkpoint completion status."""

    PENDING = "pending"
    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"


class HealthResponse(BaseModel):
    """Response from /health endpoint."""

    status: str


class ReadyCheck(BaseModel):
    """Individual readiness check result."""

    status: str
    message: str | None = None


class ReadyResponse(BaseModel):
    """Response from /ready endpoint."""

    status: str
    checks: dict[str, ReadyCheck] = Field(default_factory=dict)


class VersionResponse(BaseModel):
    """Response from /version endpoint."""

    model_config = ConfigDict(populate_by_name=True)

    version: str
    commit: str
    build_time: str = Field(alias="buildTime")


class PodVM(BaseModel):
    """Virtual machine within a pod."""

    model_config = ConfigDict(populate_by_name=True)

    name: str
    platform_id: str = Field(alias="platformId")
    platform: str = "proxmox"
    node: str = "pve"
    status: str
    ip_address: str | None = Field(None, alias="ipAddress")


class Pod(BaseModel):
    """Lab pod instance."""

    model_config = ConfigDict(populate_by_name=True)

    id: str
    lab_template_id: str = Field(alias="labTemplateId")
    lab_template: str = Field("", alias="labTemplate")
    platform: str = "proxmox"
    owner_id: str = Field(alias="ownerId")
    owner: str = ""
    status: str
    vms: list[PodVM] = Field(default_factory=list)
    networks: list[Any] = Field(default_factory=list)
    created_at: datetime = Field(alias="createdAt")
    expires_at: datetime | None = Field(None, alias="expiresAt")


class PodListResponse(BaseModel):
    """Response from GET /pods."""

    pods: list[Pod] = Field(default_factory=list)


class CreatePodResponse(BaseModel):
    """Response from POST /pods."""

    id: str
    status: str


class Session(BaseModel):
    """Lab session."""

    model_config = ConfigDict(populate_by_name=True)

    id: str
    pod_id: str = Field(alias="podId")
    user_id: str = Field(alias="userId")
    lab_template_id: str = Field(alias="labTemplateId")
    status: str = ""
    earned_points: int = Field(0, alias="earnedPoints")
    max_points: int = Field(0, alias="maxPoints")
    percentage: float = 0.0
    passed: bool = False
    passing_threshold: int = Field(0, alias="passingThreshold")
    started_at: datetime = Field(alias="startedAt")
    ended_at: datetime | None = Field(None, alias="endedAt")


class SessionListResponse(BaseModel):
    """Response from GET /sessions."""

    count: int = 0
    sessions: list[Session] = Field(default_factory=list)


class CreateSessionResponse(BaseModel):
    """Response from POST /sessions."""

    model_config = ConfigDict(populate_by_name=True)

    session_id: str = Field(alias="sessionId")
    status: str
    max_points: int = Field(alias="maxPoints")


class CheckpointProgress(BaseModel):
    """Progress on a single checkpoint."""

    model_config = ConfigDict(populate_by_name=True)

    id: str = Field(alias="id")
    name: str = Field("", alias="name")
    status: str = Field(alias="status")
    score: int = Field(0, alias="score")
    max_score: int = Field(0, alias="maxScore")


class SessionProgress(BaseModel):
    """Session progress with checkpoint details."""

    model_config = ConfigDict(populate_by_name=True)

    session_id: str = Field(alias="sessionId")
    earned_points: int = Field(alias="earnedPoints")
    max_points: int = Field(alias="maxPoints")
    percentage: float
    checkpoints: list[CheckpointProgress] = Field(default_factory=list)


class SubmitResponse(BaseModel):
    """Response from POST /sessions/{id}/submit."""

    model_config = ConfigDict(populate_by_name=True)

    session_id: str = Field(alias="sessionId")
    status: str
    earned_points: int = Field(alias="earnedPoints")
    max_points: int = Field(alias="maxPoints")
    percentage: float
    passed: bool
    checkpoints: list[dict[str, Any]] = Field(default_factory=list)


class LabTemplate(BaseModel):
    """Lab template definition."""

    model_config = ConfigDict(populate_by_name=True)

    id: str
    name: str
    slug: str = ""
    description: str = ""
    platform: str = "proxmox"
    is_active: bool = Field(True, alias="isActive")


class LabTemplateListResponse(BaseModel):
    """Response from GET /labs."""

    labs: list[LabTemplate] = Field(default_factory=list)


class User(BaseModel):
    """Authenticated user info."""

    id: str
    email: str
    name: str
    roles: list[str] = Field(default_factory=list)


class LoginResponse(BaseModel):
    """Response from POST /auth/login."""

    model_config = ConfigDict(populate_by_name=True)

    token: str
    user: User
    must_change_password: bool = Field(False, alias="mustChangePassword")

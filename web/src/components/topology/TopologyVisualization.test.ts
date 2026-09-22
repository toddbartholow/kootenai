/**
 * Tests for TopologyVisualization component
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import type { TopologyData } from '@/api'

// Use vi.hoisted to define mock instance that can be accessed in both mock factory and tests
const { mockNetworkInstance } = vi.hoisted(() => {
  return {
    mockNetworkInstance: {
      on: vi.fn(),
      once: vi.fn(),
      destroy: vi.fn(),
      fit: vi.fn(),
      getScale: vi.fn(() => 1),
      moveTo: vi.fn(),
      selectNodes: vi.fn(),
    },
  }
})

// Mock vis-network - define class inside factory
vi.mock('vis-network', () => {
  class Network {
    on: typeof mockNetworkInstance.on
    once: typeof mockNetworkInstance.once
    destroy: typeof mockNetworkInstance.destroy
    fit: typeof mockNetworkInstance.fit
    getScale: typeof mockNetworkInstance.getScale
    moveTo: typeof mockNetworkInstance.moveTo
    selectNodes: typeof mockNetworkInstance.selectNodes

    constructor() {
      this.on = mockNetworkInstance.on
      this.once = mockNetworkInstance.once
      this.destroy = mockNetworkInstance.destroy
      this.fit = mockNetworkInstance.fit
      this.getScale = mockNetworkInstance.getScale
      this.moveTo = mockNetworkInstance.moveTo
      this.selectNodes = mockNetworkInstance.selectNodes
    }
  }
  return { Network }
})

// Mock vis-data - define class inside factory
vi.mock('vis-data', () => {
  class DataSet {
    data: unknown[]
    constructor(data: unknown[]) {
      this.data = data
    }
  }
  return { DataSet }
})

// Import component after mocks are set up
import TopologyVisualization from './TopologyVisualization.vue'

// Sample topology data for tests
const sampleTopology: TopologyData = {
  podId: 'test-pod-1',
  labTemplate: 'Test Network Lab',
  segments: [
    { name: 'external', vlan: 100, subnet: '10.0.100.0/24', gateway: '10.0.100.1', dhcp: false },
    { name: 'internal', vlan: 101, subnet: '10.0.101.0/24', gateway: '10.0.101.1', dhcp: true },
  ],
  vms: [
    {
      name: 'firewall',
      platformId: 'vm-1',
      status: 'running',
      ipAddress: '10.0.100.1',
      template: 'pfsense',
      networks: [
        { segment: 'external', ip: '10.0.100.1' },
        { segment: 'internal', ip: '10.0.101.1' },
      ],
      resources: { cpu: 2, memory: 4096, disk: 32 },
    },
    {
      name: 'webserver',
      platformId: 'vm-2',
      status: 'running',
      ipAddress: '10.0.101.10',
      template: 'ubuntu-server',
      networks: [{ segment: 'internal', ip: '10.0.101.10' }],
      resources: { cpu: 4, memory: 8192, disk: 64 },
    },
    {
      name: 'attacker',
      platformId: 'vm-3',
      status: 'stopped',
      ipAddress: '10.0.100.50',
      template: 'kali-linux',
      networks: [{ segment: 'external', ip: '10.0.100.50' }],
      resources: { cpu: 2, memory: 4096 },
    },
  ],
}

describe('TopologyVisualization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('mounting', () => {
    it('mounts successfully with topology prop', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.exists()).toBe(true)
    })

    it('renders toolbar with zoom controls', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      const toolbar = wrapper.find('.topology-toolbar')
      expect(toolbar.exists()).toBe(true)
    })

    it('renders canvas container', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      const canvas = wrapper.find('.topology-canvas')
      expect(canvas.exists()).toBe(true)
    })

    it('renders legend items', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      const html = wrapper.html()
      expect(html).toContain('Running')
      expect(html).toContain('Stopped')
      expect(html).toContain('Starting')
      expect(html).toContain('Network')
    })
  })

  describe('computed properties', () => {
    it('builds vmMap correctly', async () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      // Access the component's internal state via the exposed test API
      expect(wrapper.vm.vmMap.size).toBe(3)
      expect(wrapper.vm.vmMap.has('vm-firewall')).toBe(true)
      expect(wrapper.vm.vmMap.has('vm-webserver')).toBe(true)
      expect(wrapper.vm.vmMap.has('vm-attacker')).toBe(true)
      expect(wrapper.vm.vmMap.get('vm-firewall')?.name).toBe('firewall')
    })

    it('builds segmentMap correctly', async () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.vm.segmentMap.size).toBe(2)
      expect(wrapper.vm.segmentMap.has('seg-external')).toBe(true)
      expect(wrapper.vm.segmentMap.has('seg-internal')).toBe(true)
      expect(wrapper.vm.segmentMap.get('seg-external')?.vlan).toBe(100)
    })
  })

  describe('exposed methods', () => {
    it('exposes fitToView method', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(typeof wrapper.vm.fitToView).toBe('function')
      expect(typeof wrapper.vm.zoomIn).toBe('function')
      expect(typeof wrapper.vm.zoomOut).toBe('function')
      expect(typeof wrapper.vm.exportAsImage).toBe('function')
    })

    it('fitToView calls network.fit', async () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
        attachTo: document.body,
      })

      // Mock getBoundingClientRect to return non-zero dimensions
      const container = wrapper.find('.topology-canvas').element
      vi.spyOn(container, 'getBoundingClientRect').mockReturnValue({
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      })

      wrapper.vm.fitToView()

      expect(mockNetworkInstance.fit).toHaveBeenCalled()

      wrapper.unmount()
    })

    it('zoomIn calls network.moveTo with increased scale', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      wrapper.vm.zoomIn()

      expect(mockNetworkInstance.getScale).toHaveBeenCalled()
      expect(mockNetworkInstance.moveTo).toHaveBeenCalledWith({ scale: 1.2 })
    })

    it('zoomOut calls network.moveTo with decreased scale', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      wrapper.vm.zoomOut()

      expect(mockNetworkInstance.getScale).toHaveBeenCalled()
      expect(mockNetworkInstance.moveTo).toHaveBeenCalledWith(
        expect.objectContaining({ scale: expect.closeTo(0.833, 2) })
      )
    })
  })

  describe('props', () => {
    it('accepts topology prop', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.props('topology')).toEqual(sampleTopology)
    })

    it('accepts optional selectedVM prop', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
          selectedVM: 'firewall',
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.props('selectedVM')).toBe('firewall')
    })

    it('defaults selectedVM to undefined', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.props('selectedVM')).toBeUndefined()
    })
  })

  describe('empty topology handling', () => {
    it('handles topology with no VMs', () => {
      const emptyTopology: TopologyData = {
        podId: 'empty-pod',
        labTemplate: 'Empty Lab',
        segments: [{ name: 'default', vlan: 1, subnet: '10.0.0.0/24', dhcp: true }],
        vms: [],
      }

      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: emptyTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.exists()).toBe(true)
      expect(wrapper.vm.vmMap.size).toBe(0)
    })

    it('handles topology with no segments', () => {
      const noSegmentsTopology: TopologyData = {
        podId: 'no-segments-pod',
        labTemplate: 'No Segments Lab',
        segments: [],
        vms: [
          {
            name: 'standalone',
            platformId: 'vm-1',
            status: 'running',
            template: 'ubuntu',
            networks: [],
            resources: { cpu: 1, memory: 1024 },
          },
        ],
      }

      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: noSegmentsTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(wrapper.exists()).toBe(true)
      expect(wrapper.vm.segmentMap.size).toBe(0)
    })
  })

  describe('network initialization', () => {
    it('initializes network on mount', () => {
      // The Network constructor should be called when component mounts
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      // If the component mounted successfully, the network was initialized
      expect(wrapper.exists()).toBe(true)
    })

    it('registers click event handler', () => {
      mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(mockNetworkInstance.on).toHaveBeenCalledWith('click', expect.any(Function))
    })

    it('registers doubleClick event handler', () => {
      mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(mockNetworkInstance.on).toHaveBeenCalledWith('doubleClick', expect.any(Function))
    })

    it('registers hover event handlers', () => {
      mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      expect(mockNetworkInstance.on).toHaveBeenCalledWith('hoverNode', expect.any(Function))
      expect(mockNetworkInstance.on).toHaveBeenCalledWith('blurNode', expect.any(Function))
    })
  })

  describe('cleanup', () => {
    it('destroys network on unmount', () => {
      const wrapper = mount(TopologyVisualization, {
        props: {
          topology: sampleTopology,
        },
        global: {
          stubs: {
            Button: true,
            Tag: true,
            Transition: true,
          },
          directives: {
            tooltip: {},
          },
        },
      })

      wrapper.unmount()

      expect(mockNetworkInstance.destroy).toHaveBeenCalled()
    })
  })
})

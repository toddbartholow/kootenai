import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  guideSidebar: [
    'index',
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: ['getting-started', 'quickstart-demo', 'guides/prerequisites'],
    },
    {
      type: 'category',
      label: 'User Guide',
      items: [
        'guides/overview',
        'tutorials/first-lab',
        'guides/working-with-pods',
        'guides/sessions-progress',
        'achievement-system',
      ],
    },
    {
      type: 'category',
      label: 'Instructor Guide',
      items: [
        'instructor/guide',
        'lab-templates/template-creation-guide',
        'instructor/managing-students',
        'instructor/grading',
      ],
    },
    {
      type: 'category',
      label: 'Simulation',
      items: [
        'guides/classroom-simulation',
      ],
    },
    'glossary',
    'enterprise',
  ],

  apiSidebar: [
    'api/reference',
    'api/authentication',
    'api/labs-pods',
    'api/sessions',
    'api/achievements',
    'api/organizations',
    'api/websocket',
    'api/classroom-simulation',
    'api/openapi',
  ],

  adminSidebar: [
    {
      type: 'category',
      label: 'Deployment',
      collapsed: false,
      items: [
        'admin/production-deployment',
        'admin/proxmox-setup',
        'admin/proxmox-vm-templates',
        'admin/scaling',
      ],
    },
    {
      type: 'category',
      label: 'Integrations',
      items: [
        'admin/canvas-lms-integration',
        'admin/wazuh-integration',
        'admin/wazuh-security-hardening',
        'admin/secret-rotation',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/frontend',
        'architecture/platform-comparison',
        'architecture/objective-tracking',
        'architecture/async-job-processing',
        'architecture/multi-tenancy-plan',
        {
          type: 'category',
          label: 'Decision Records',
          items: ['architecture/adr/golang-choice', 'architecture/adr/postgresql-choice'],
        },
      ],
    },
    {
      type: 'category',
      label: 'Database',
      items: [
        'reference/database-schema',
        'reference/migrations',
        'reference/query-patterns',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/mage',
        'development/ci-cd',
        'development/testing',
        'development/version-tracking',
        'development/contributing',
        'development/ce-ee-setup',
        'development/LICENSE',
      ],
    },
    {
      type: 'category',
      label: 'Templates',
      items: [
        'lab-templates/template-creation-guide',
        'lab-templates/yaml-schema',
        'lab-templates/examples',
      ],
    },
    {
      type: 'category',
      label: 'Troubleshooting',
      items: [
        'troubleshooting/common-issues',
        'troubleshooting/lti-vnc-console',
        'troubleshooting/wazuh-syscheck-parsing-fix',
      ],
    },
  ],
};

export default sidebars;

import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
  link: string;
  linkText: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'For Students',
    icon: '🎓',
    description: (
      <>
        Launch pre-configured lab environments with a single click. Practice
        cybersecurity skills in isolated, safe environments with real VMs.
        Track your progress and earn achievements as you learn.
      </>
    ),
    link: '/docs/guides/overview',
    linkText: 'User Guide',
  },
  {
    title: 'For Instructors',
    icon: '👨‍🏫',
    description: (
      <>
        Create custom lab templates with YAML definitions. Monitor student
        progress in real-time, manage lab sessions, and integrate with
        Canvas LMS for seamless grading.
      </>
    ),
    link: '/docs/instructor/guide',
    linkText: 'Instructor Guide',
  },
  {
    title: 'For Administrators',
    icon: '⚙️',
    description: (
      <>
        Deploy on Proxmox or CloudStack infrastructure. Configure multi-tenancy
        with organizations and teams. Scale horizontally with async job
        processing and NATS messaging.
      </>
    ),
    link: '/docs/admin/production-deployment',
    linkText: 'Deployment Guide',
  },
];

function Feature({title, icon, description, link, linkText}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <span style={{fontSize: '4rem'}}>{icon}</span>
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
        <Link className="button button--primary button--sm" to={link}>
          {linkText}
        </Link>
      </div>
    </div>
  );
}

function KeyFeatures(): ReactNode {
  return (
    <section className={styles.features} style={{backgroundColor: 'var(--ifm-color-emphasis-100)'}}>
      <div className="container">
        <Heading as="h2" className="text--center margin-bottom--lg">
          Platform Features
        </Heading>
        <div className="row">
          <div className="col col--6">
            <ul>
              <li><strong>Proxmox Integration</strong> - Fast snapshot/revert for security exercises</li>
              <li><strong>CloudStack Support</strong> - Multi-tenant cloud infrastructure training</li>
              <li><strong>Canvas LMS Integration</strong> - LTI 1.3 for seamless grade passback</li>
              <li><strong>Achievement System</strong> - Gamified learning with badges and points</li>
            </ul>
          </div>
          <div className="col col--6">
            <ul>
              <li><strong>Real-time Progress</strong> - WebSocket-based checkpoint tracking</li>
              <li><strong>Wazuh SIEM</strong> - Monitor lab activities and detect objectives</li>
              <li><strong>Multi-tenancy</strong> - Organizations, teams, and role-based access</li>
              <li><strong>REST API</strong> - Full API for automation and integrations</li>
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}

function QuickLinks(): ReactNode {
  const links = [
    {title: 'Getting Started', to: '/docs/getting-started', desc: 'Installation and setup'},
    {title: 'API Reference', to: '/docs/api/reference', desc: 'REST API documentation'},
    {title: 'Lab Templates', to: '/docs/lab-templates/template-creation-guide', desc: 'Create custom labs'},
    {title: 'Architecture', to: '/docs/architecture/overview', desc: 'System design overview'},
  ];

  return (
    <section className={styles.features}>
      <div className="container">
        <Heading as="h2" className="text--center margin-bottom--lg">
          Quick Links
        </Heading>
        <div className="row">
          {links.map((link, idx) => (
            <div key={idx} className="col col--3">
              <Link to={link.to} className="card padding--md" style={{display: 'block', textDecoration: 'none', height: '100%'}}>
                <Heading as="h4" style={{marginBottom: '0.5rem'}}>{link.title}</Heading>
                <p style={{marginBottom: 0, color: 'var(--ifm-color-emphasis-700)'}}>{link.desc}</p>
              </Link>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <>
      <section className={styles.features}>
        <div className="container">
          <div className="row">
            {FeatureList.map((props, idx) => (
              <Feature key={idx} {...props} />
            ))}
          </div>
        </div>
      </section>
      <KeyFeatures />
      <QuickLinks />
    </>
  );
}

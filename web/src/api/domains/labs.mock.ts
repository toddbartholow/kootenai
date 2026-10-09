/**
 * Mock data for labs API
 * Loaded dynamically only when USE_MOCK_DATA is true
 */
import type { Lab, LabInstructions } from './labs'

export const mockLabs: Lab[] = [
  {
    id: 'lab-firewall-101',
    name: 'Firewall Configuration 101',
    description:
      'Learn to configure pfSense firewall rules to protect your network. This hands-on lab covers basic allow/deny rules, NAT configuration, and firewall best practices.',
    difficulty: 'intermediate',
    durationMinutes: 45,
    platform: 'proxmox',
    tags: ['firewall', 'security', 'pfSense', 'networking'],
    version: '1.2.0',
    maxPoints: 1000,
    passThreshold: 70,
    isActive: true,
  },
  {
    id: 'lab-ids-snort',
    name: 'Intrusion Detection with Snort',
    description:
      'Deploy and configure Snort IDS to detect network intrusions. Learn to write custom rules and analyze alerts.',
    difficulty: 'advanced',
    durationMinutes: 60,
    platform: 'proxmox',
    tags: ['IDS', 'security', 'snort', 'monitoring'],
    version: '2.0.1',
    maxPoints: 1500,
    passThreshold: 75,
    isActive: true,
  },
  {
    id: 'lab-network-fundamentals',
    name: 'Network Fundamentals Review',
    description:
      'Review TCP/IP, OSI model, and network protocols. Perfect for beginners or those needing a refresher.',
    difficulty: 'beginner',
    durationMinutes: 30,
    platform: 'proxmox',
    tags: ['networking', 'tcp-ip', 'fundamentals'],
    version: '1.0.0',
    maxPoints: 800,
    passThreshold: 60,
    isActive: true,
  },
  {
    id: 'lab-vpn-config',
    name: 'VPN Configuration',
    description:
      'Set up site-to-site and remote access VPNs using OpenVPN and WireGuard. Includes certificate management.',
    difficulty: 'advanced',
    durationMinutes: 75,
    platform: 'proxmox',
    tags: ['vpn', 'security', 'openvpn', 'wireguard'],
    version: '1.5.0',
    maxPoints: 1500,
    passThreshold: 70,
    isActive: true,
  },
  {
    id: 'lab-siem-basics',
    name: 'Security Monitoring & SIEM',
    description:
      'Configure centralized logging and security monitoring with Wazuh. Learn log aggregation and alert correlation.',
    difficulty: 'advanced',
    durationMinutes: 90,
    platform: 'proxmox',
    tags: ['SIEM', 'security', 'wazuh', 'logging'],
    version: '1.1.0',
    maxPoints: 1500,
    passThreshold: 75,
    isActive: true,
  },
  {
    id: 'lab-cloud-aws',
    name: 'AWS Security Fundamentals',
    description:
      'Learn AWS security best practices including IAM, VPC security groups, and CloudTrail logging.',
    difficulty: 'intermediate',
    durationMinutes: 60,
    platform: 'cloudstack',
    tags: ['cloud', 'aws', 'security', 'IAM'],
    version: '2.1.0',
    maxPoints: 1200,
    passThreshold: 70,
    isActive: true,
  },
  {
    id: 'lab-incident-response',
    name: 'Incident Response Basics',
    description:
      'Learn incident detection and response procedures. Includes forensic analysis and containment strategies.',
    difficulty: 'advanced',
    durationMinutes: 80,
    platform: 'proxmox',
    tags: ['incident-response', 'forensics', 'security'],
    version: '1.3.0',
    maxPoints: 1400,
    passThreshold: 70,
    isActive: true,
  },
  {
    id: 'lab-windows-hardening',
    name: 'Windows Server Hardening',
    description: 'Secure Windows Server environments following CIS benchmarks and best practices.',
    difficulty: 'intermediate',
    durationMinutes: 50,
    platform: 'proxmox',
    tags: ['windows', 'hardening', 'security', 'CIS'],
    version: '1.0.2',
    maxPoints: 1000,
    passThreshold: 70,
    isActive: true,
  },
]

// Mock instructions for development
export const mockInstructions: Record<string, LabInstructions> = {
  'simple-linux-intro': {
    overview: `Welcome to your first Linux lab! In this exercise, you'll learn the fundamental
commands that every Linux user needs to know. By the end, you'll be comfortable
creating directories, files, and understanding how the Linux filesystem works.`,
    learning_objectives: [
      'Understand the Linux directory structure and home directory concept',
      'Create and organize directories using the mkdir command',
      'Create and manipulate files using touch and echo',
      'View filesystem information using ls and df commands',
      'Understand file paths (absolute vs relative)',
    ],
    prerequisites: ['Basic computer literacy', 'Access to a terminal (provided in this lab)'],
    steps: [
      {
        id: 'step-1',
        title: 'Understanding Your Home Directory',
        objective_id: 'create-directory',
        content: `## What is a Home Directory?

In Linux, every user has a personal space called a **home directory**. This is
where you store your personal files, configurations, and projects. It's similar
to "My Documents" in Windows or your user folder on a Mac.

Your home directory path is \`/home/student\` (or \`/home/<your-username>\` on other
systems). Linux provides a convenient shortcut: the tilde symbol \`~\` always
refers to your home directory.

## Why Organize with Directories?

Just like physical folders help organize papers on your desk, directories (folders)
in Linux help organize your files. Good organization makes it easier to:
- Find files quickly
- Back up important data
- Share specific projects with others
- Keep your system clean and manageable

## Creating Your First Directory

The \`mkdir\` command (short for "make directory") creates new directories.

**Syntax:** \`mkdir [options] directory_name\`

### Your Task

Create a directory called \`mywork\` in your home directory:

\`\`\`bash
mkdir ~/mywork
\`\`\`

**Breaking it down:**
- \`mkdir\` - the command to create a directory
- \`~/mywork\` - the path where you want to create it
  - \`~\` expands to \`/home/student\`
  - So this creates \`/home/student/mywork\`

### Verify It Worked

After creating the directory, you can verify it exists:

\`\`\`bash
ls ~
\`\`\`

You should see \`mywork\` listed in the output.`,
      },
      {
        id: 'step-2',
        title: 'Creating an Empty File',
        objective_id: 'create-file',
        content: `## What is the touch Command?

The \`touch\` command is one of the most versatile commands in Linux. Its primary
purposes are:

1. **Create empty files** - If a file doesn't exist, \`touch\` creates it
2. **Update timestamps** - If a file exists, \`touch\` updates its modification time

In this step, we'll use it to create a new, empty file.

## Creating notes.txt

**Syntax:** \`touch file_path\`

### Your Task

Create a file called \`notes.txt\` inside your \`mywork\` directory:

\`\`\`bash
touch ~/mywork/notes.txt
\`\`\`

### Verify the File Exists

\`\`\`bash
ls ~/mywork
\`\`\`

You should see \`notes.txt\` in the output.`,
      },
      {
        id: 'step-3',
        title: 'Writing Content to a File',
        objective_id: 'write-content',
        content: `## Output Redirection: The Power of >

Linux has a powerful feature called **output redirection**. Normally, when you
run a command, its output appears on your screen (called "standard output" or stdout).
The \`>\` operator redirects this output to a file instead.

## The echo Command

\`echo\` simply prints (echoes) text to standard output. Combined with redirection,
it becomes a quick way to write content to files.

**Examples:**
- \`echo "Hello"\` - prints "Hello" to screen
- \`echo "Hello" > file.txt\` - writes "Hello" to file.txt

## Overwrite vs Append

| Operator | Behavior | Example |
|----------|----------|---------|
| \`>\` | **Overwrites** the file | \`echo "new" > file\` |
| \`>>\` | **Appends** to the file | \`echo "more" >> file\` |

### Your Task

Write the text "Hello, Kootenai!" into your notes.txt file:

\`\`\`bash
echo "Hello, Kootenai!" > ~/mywork/notes.txt
\`\`\`

### Verify the Content

\`\`\`bash
cat ~/mywork/notes.txt
\`\`\``,
      },
      {
        id: 'step-4',
        title: 'Checking Disk Space',
        objective_id: 'check-disk-space',
        content: `## Why Monitor Disk Space?

Running out of disk space can cause serious problems. Regularly checking disk
space is a good habit for any Linux user or administrator.

## The df Command

\`df\` stands for "disk free" and shows how much space is available on mounted
filesystems.

**Common options:**
- \`df\` - basic output in 1K blocks
- \`df -h\` - **human-readable** format (KB, MB, GB)
- \`df -T\` - shows filesystem **type**

### Your Task

Check the available disk space on your system:

\`\`\`bash
df -h
\`\`\`

### Understanding the Output

| Column | Meaning |
|--------|---------|
| Filesystem | The device or partition name |
| Size | Total capacity |
| Used | Space currently used |
| Avail | Free space remaining |
| Use% | Percentage used |
| Mounted on | Where it's accessible |`,
      },
      {
        id: 'step-5',
        title: 'Listing Directory Contents',
        objective_id: 'list-files',
        content: `## The ls Command: Your Filesystem Explorer

\`ls\` (list) is probably the most frequently used Linux command. It shows you
what files and directories exist in a location.

## Common ls Options

| Option | Purpose |
|--------|---------|
| (none) | Basic listing |
| \`-l\` | Long format (details) |
| \`-a\` | Show hidden files |
| \`-h\` | Human-readable sizes |
| \`-R\` | Recursive |

**Pro tip:** Combine options! \`ls -lah\` shows all files in long, human-readable format.

### Your Task

List the contents of your mywork directory:

\`\`\`bash
ls ~/mywork
\`\`\`

For more details, try:
\`\`\`bash
ls -l ~/mywork
\`\`\``,
      },
    ],
    summary: `## Congratulations!

You've completed the Simple Linux Introduction lab. You've learned:

1. **Home Directory (\`~\`)** - Your personal space in the Linux filesystem
2. **mkdir** - Create directories to organize your files
3. **touch** - Create empty files or update timestamps
4. **echo with >** - Write content to files using output redirection
5. **df -h** - Check available disk space in human-readable format
6. **ls** - List and explore directory contents

Continue to the **Linux Foundations** lab to deepen your knowledge!`,
    tips: [
      "Use Tab for auto-completion: type 'cd ~/myw' and press Tab to complete to 'mywork'",
      'Use the up arrow key to recall previous commands',
      "If you make a mistake, you can always delete and start over with 'rm -r ~/mywork'",
      "The 'clear' command cleans up your terminal screen",
      "Use 'man <command>' to read the manual for any command (e.g., 'man ls')",
    ],
    resources: [
      {
        title: 'Linux Command Line Basics',
        url: 'https://ubuntu.com/tutorials/command-line-for-beginners',
      },
      { title: 'The Linux Filesystem Hierarchy', url: 'https://www.pathname.com/fhs/' },
    ],
  },
}

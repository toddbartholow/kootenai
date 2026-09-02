-- Migration: Add educational instructions for Linux Foundations lab
-- This migration populates step-by-step learning content for the beginner Linux lab.

UPDATE lab_templates
SET instructions = '{
  "overview": "Welcome to Linux Foundations! This lab will teach you the essential commands every Linux user needs to know. You''ll learn to navigate the filesystem, understand directory structures, create files and folders, and get introduced to two powerful text editors: nano and vim.\n\nLinux powers most of the internet''s servers, cloud infrastructure, and countless devices. Understanding the command line is a fundamental skill that opens doors to system administration, DevOps, cybersecurity, and software development.",
  "learning_objectives": [
    "Understand the Linux filesystem hierarchy and how directories are organized",
    "Navigate the filesystem using cd, pwd, and understand absolute vs relative paths",
    "List directory contents with ls and interpret file permissions, sizes, and timestamps",
    "Create directories including nested directory structures with mkdir",
    "Create and edit files using both nano (beginner-friendly) and vim (powerful)"
  ],
  "prerequisites": [
    "Basic computer literacy (keyboard, mouse, typing)",
    "Access to a terminal (provided in this lab)",
    "No prior Linux experience required - this is a beginner lab!"
  ],
  "steps": [
    {
      "id": "step-1",
      "title": "Understanding Your Location with pwd",
      "objective_id": "pwd-command",
      "content": "## Where Am I?\n\nWhen you first open a terminal, you''re placed somewhere in the filesystem. But where? The `pwd` command (Print Working Directory) tells you exactly where you are.\n\n### The Linux Filesystem\n\nUnlike Windows with its drive letters (C:, D:), Linux has a single tree structure starting from the **root** directory, represented by `/`. Everything branches from there:\n\n```\n/                    <- Root (top of everything)\n├── home/            <- User home directories\n│   └── student/     <- Your home directory\n├── etc/             <- System configuration files\n├── var/             <- Variable data (logs, databases)\n├── usr/             <- User programs and utilities\n└── tmp/             <- Temporary files\n```\n\n### Your Home Directory\n\nEvery user has a home directory at `/home/username`. This is your personal space for files, configurations, and projects. The tilde symbol `~` is a shortcut that always refers to your home directory.\n\n### Your Task\n\nRun the `pwd` command to see your current location:\n\n```bash\npwd\n```\n\nYou should see something like `/home/student` - this is your home directory, your starting point in the filesystem."
    },
    {
      "id": "step-2",
      "title": "Listing Directory Contents with ls",
      "objective_id": "list-home",
      "content": "## What''s Here?\n\nNow that you know where you are, let''s see what''s in the current directory. The `ls` command (list) shows the contents of a directory.\n\n### Basic Usage\n\n```bash\nls          # List current directory\nls ~        # List home directory (same thing if you''re home)\nls /etc     # List a specific directory\n```\n\n### Why This Matters\n\nBeing able to quickly see what files and directories exist is fundamental to navigating any system. Whether you''re:\n- Looking for configuration files\n- Finding where logs are stored\n- Exploring a new codebase\n- Checking what backups exist\n\n`ls` is always your first step.\n\n### Your Task\n\nList your home directory contents:\n\n```bash\nls\n```\n\nYou might see existing files or an empty directory - both are normal for a fresh account!"
    },
    {
      "id": "step-3",
      "title": "Detailed File Listings with ls -l",
      "objective_id": "list-detailed",
      "content": "## Getting More Information\n\nThe basic `ls` shows filenames, but often you need more details. The `-l` flag (long format) reveals crucial information about each file.\n\n### Understanding the Output\n\n```\n-rw-r--r-- 1 student student  1234 Dec 30 10:00 example.txt\n│└──┬───┘    │       │       │    │            │\n│   │        │       │       │    │            └─ Filename\n│   │        │       │       │    └─ Last modified date/time\n│   │        │       │       └─ File size in bytes\n│   │        │       └─ Group owner\n│   │        └─ File owner\n│   └─ Permissions (read/write/execute for owner/group/others)\n└─ File type (- = file, d = directory, l = link)\n```\n\n### Permissions Explained\n\nThe permission string `rw-r--r--` breaks into three groups:\n- **Owner** (rw-): can read and write\n- **Group** (r--): can only read\n- **Others** (r--): can only read\n\n### Your Task\n\nRun ls with the long format option:\n\n```bash\nls -l\n```\n\n**Pro tip:** You can combine options! `ls -lh` adds human-readable file sizes (KB, MB instead of bytes)."
    },
    {
      "id": "step-4",
      "title": "Finding Hidden Files with ls -a",
      "objective_id": "list-hidden",
      "content": "## The Hidden World\n\nIn Linux, files starting with a dot (`.`) are hidden by default. These are typically configuration files that you don''t need to see day-to-day, but they''re important!\n\n### Common Hidden Files\n\n| File | Purpose |\n|------|--------|\n| `.bashrc` | Shell configuration (aliases, prompt settings) |\n| `.profile` | Login shell settings |\n| `.ssh/` | SSH keys and configuration |\n| `.gitconfig` | Git user settings |\n| `.vimrc` | Vim editor configuration |\n\n### Why Hide Files?\n\nHiding configuration files keeps your directory clean and focused on your actual work files. You don''t want to accidentally delete `.bashrc` when cleaning up!\n\n### Special Entries\n\nYou''ll also see:\n- `.` (single dot) - Current directory\n- `..` (double dot) - Parent directory\n\nThese are how Linux represents \"here\" and \"up one level.\"\n\n### Your Task\n\nShow all files including hidden ones:\n\n```bash\nls -a\n```\n\n**Pro tip:** Combine with long format: `ls -la` shows hidden files with full details."
    },
    {
      "id": "step-5",
      "title": "Navigating to System Directories",
      "objective_id": "navigate-etc",
      "content": "## Moving Around\n\nThe `cd` command (change directory) moves you to a different location in the filesystem.\n\n### Path Types\n\n**Absolute paths** start from root (`/`):\n```bash\ncd /etc              # Go to /etc from anywhere\ncd /home/student     # Go to your home directory\n```\n\n**Relative paths** are relative to where you are:\n```bash\ncd projects          # Go to projects/ in current directory\ncd ../               # Go up one level\ncd ../../            # Go up two levels\n```\n\n### The /etc Directory\n\nThe `/etc` directory contains system-wide configuration files. Some important ones:\n\n| File | Purpose |\n|------|--------|\n| `/etc/passwd` | User account information |\n| `/etc/hosts` | Hostname to IP mappings |\n| `/etc/ssh/sshd_config` | SSH server configuration |\n| `/etc/fstab` | Filesystem mount points |\n\n### Your Task\n\nNavigate to the /etc directory:\n\n```bash\ncd /etc\n```\n\nAfter running this, use `pwd` to confirm you''re in `/etc`, then `ls` to explore the configuration files. Don''t worry - you can''t break anything just by looking!"
    },
    {
      "id": "step-6",
      "title": "Returning Home",
      "objective_id": "navigate-home",
      "content": "## Going Home\n\nNo matter where you are in the filesystem, you can always get back home quickly.\n\n### Three Ways Home\n\n```bash\ncd ~         # Tilde expands to your home directory\ncd           # cd with no arguments goes home\ncd $HOME     # $HOME is an environment variable with your home path\n```\n\nAll three do exactly the same thing - use whichever feels natural!\n\n### The Tilde (~) Shortcut\n\nThe tilde is incredibly useful in paths:\n```bash\ncd ~/projects        # Go to projects in home directory\nls ~/.ssh            # List SSH directory in home\ncp file.txt ~/       # Copy file to home directory\n```\n\n### Your Task\n\nReturn to your home directory:\n\n```bash\ncd ~\n```\n\nOr simply:\n```bash\ncd\n```\n\nUse `pwd` to confirm you''re back at `/home/student`."
    },
    {
      "id": "step-7",
      "title": "Creating Directories with mkdir",
      "objective_id": "create-projects",
      "content": "## Building Your Workspace\n\nThe `mkdir` command (make directory) creates new directories. Organizing your work into directories is a fundamental skill.\n\n### Basic Syntax\n\n```bash\nmkdir directory_name\nmkdir ~/projects                    # Create in home directory\nmkdir /tmp/test                     # Create using absolute path\n```\n\n### Best Practices for Directory Names\n\n- Use lowercase letters\n- Use hyphens or underscores instead of spaces: `my-project` or `my_project`\n- Be descriptive but concise\n- Avoid special characters except `-` and `_`\n\n### Why Organize?\n\nGood directory structure makes projects manageable:\n```\n~/projects/\n├── web-app/\n│   ├── src/\n│   ├── tests/\n│   └── docs/\n├── scripts/\n└── backups/\n```\n\n### Your Task\n\nCreate a projects directory in your home:\n\n```bash\nmkdir ~/projects\n```\n\nVerify it exists:\n```bash\nls ~\n```\n\nYou should now see `projects` in the listing."
    },
    {
      "id": "step-8",
      "title": "Creating Nested Directories",
      "objective_id": "create-nested",
      "content": "## Creating Directory Trees\n\nWhat if you want to create `projects/lab1/data` but `lab1` doesn''t exist yet? Without special options, `mkdir` would fail.\n\n### The -p Flag (Parents)\n\nThe `-p` flag creates parent directories as needed:\n\n```bash\nmkdir -p ~/projects/lab1/data\n```\n\nThis single command creates:\n1. `lab1/` inside `projects/` (if it doesn''t exist)\n2. `data/` inside `lab1/`\n\n### Why -p Is Useful\n\n- **Idempotent**: Running it multiple times doesn''t cause errors\n- **Efficient**: Create deep structures in one command\n- **Scripts**: Essential for automation scripts that set up directory structures\n\n### Without -p\n\n```bash\nmkdir ~/projects/newdir/subdir    # ERROR: newdir doesn''t exist!\n```\n\n### Your Task\n\nCreate a nested directory structure in one command:\n\n```bash\nmkdir -p ~/projects/lab1/data\n```\n\nVerify the structure:\n```bash\nls ~/projects\nls ~/projects/lab1\n```"
    },
    {
      "id": "step-9",
      "title": "Creating Files with nano",
      "objective_id": "create-with-nano",
      "content": "## The Nano Editor\n\nNano is a beginner-friendly text editor that runs in the terminal. It shows helpful keyboard shortcuts at the bottom of the screen.\n\n### Opening Nano\n\n```bash\nnano filename.txt     # Open or create a file\nnano ~/projects/readme.txt\n```\n\n### Essential Nano Commands\n\nThe `^` symbol means hold `Ctrl`:\n\n| Shortcut | Action |\n|----------|--------|\n| `^O` | Save file (Write Out) |\n| `^X` | Exit nano |\n| `^K` | Cut current line |\n| `^U` | Paste cut text |\n| `^W` | Search |\n| `^G` | Help |\n\n### The Save and Exit Flow\n\n1. Type your content\n2. Press `Ctrl+O` to save\n3. Press `Enter` to confirm the filename\n4. Press `Ctrl+X` to exit\n\n### Your Task\n\nCreate a readme file in your projects directory:\n\n```bash\nnano ~/projects/readme.txt\n```\n\n1. Type something like: `This is my projects directory`\n2. Press `Ctrl+O` then `Enter` to save\n3. Press `Ctrl+X` to exit\n\nVerify the file:\n```bash\ncat ~/projects/readme.txt\n```"
    },
    {
      "id": "step-10",
      "title": "Bonus: Creating Files with vim",
      "objective_id": "vim-file",
      "content": "## The Vim Editor\n\nVim is a powerful, modal text editor favored by many developers and sysadmins. It has a steeper learning curve but offers incredible efficiency once mastered.\n\n### Modal Editing\n\nVim has different **modes**:\n\n| Mode | Purpose | How to Enter |\n|------|---------|-------------|\n| Normal | Navigation, commands | Press `Esc` |\n| Insert | Typing text | Press `i` |\n| Command | Save, quit, search | Press `:` (from Normal) |\n\n### Essential Vim Workflow\n\n1. **Open file**: `vim filename.txt`\n2. **Enter Insert mode**: Press `i`\n3. **Type your content**\n4. **Return to Normal mode**: Press `Esc`\n5. **Save and quit**: Type `:wq` and press `Enter`\n\n### Critical Commands\n\n| Command | Action |\n|---------|--------|\n| `i` | Insert mode (start typing) |\n| `Esc` | Return to Normal mode |\n| `:w` | Save (write) |\n| `:q` | Quit |\n| `:wq` | Save and quit |\n| `:q!` | Quit without saving |\n\n### Your Task\n\nCreate a notes file using vim:\n\n```bash\nvim ~/notes.txt\n```\n\n1. Press `i` to enter Insert mode\n2. Type: `My first vim file!`\n3. Press `Esc` to return to Normal mode\n4. Type `:wq` and press `Enter` to save and quit\n\n**Stuck in vim?** Press `Esc` several times, then type `:q!` and press `Enter` to force quit without saving."
    }
  ],
  "summary": "## Congratulations!\n\nYou''ve completed the Linux Foundations lab and learned essential skills:\n\n### Commands Mastered\n\n| Command | Purpose |\n|---------|--------|\n| `pwd` | Print working directory |\n| `ls` | List directory contents |\n| `ls -l` | Long listing with details |\n| `ls -a` | Show hidden files |\n| `cd` | Change directory |\n| `mkdir` | Create directories |\n| `mkdir -p` | Create nested directories |\n| `nano` | Beginner-friendly editor |\n| `vim` | Powerful modal editor |\n\n### Key Concepts\n\n- **Root directory** (`/`) is the top of the filesystem\n- **Home directory** (`~`) is your personal space\n- **Hidden files** start with a dot (`.`)\n- **Absolute paths** start from `/`\n- **Relative paths** start from current location\n\n### Next Steps\n\nNow that you understand the basics, you''re ready for:\n- Shell Essentials (pipes, redirection, environment variables)\n- File Mastery (permissions, ownership, links)\n- Text Processing (grep, awk, sed)",
  "tips": [
    "Use Tab for auto-completion - it saves time and prevents typos!",
    "Use the up arrow to recall previous commands",
    "Use Ctrl+C to cancel a running command or get out of trouble",
    "Use Ctrl+L or ''clear'' to clean up your terminal screen",
    "Use ''man command'' to read the manual for any command (e.g., ''man ls'')",
    "Create a cheat sheet file in your home directory as you learn new commands"
  ],
  "resources": [
    {"title": "Linux Command Line Basics - Ubuntu Tutorial", "url": "https://ubuntu.com/tutorials/command-line-for-beginners"},
    {"title": "The Linux Filesystem Hierarchy Standard", "url": "https://refspecs.linuxfoundation.org/FHS_3.0/fhs-3.0.html"},
    {"title": "Vim Adventures - Learn Vim While Playing a Game", "url": "https://vim-adventures.com/"},
    {"title": "Linux Journey - Free Linux Learning Resource", "url": "https://linuxjourney.com/"}
  ]
}'::jsonb
WHERE name = 'Linux Foundations';

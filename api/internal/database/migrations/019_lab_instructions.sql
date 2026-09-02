-- Migration: Add instructions support to lab_templates table
-- This migration adds a JSONB column to store educational content for labs.

-- Add instructions column to lab_templates table
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS instructions JSONB;

-- Add comment for documentation
COMMENT ON COLUMN lab_templates.instructions IS 'Educational content including overview, learning objectives, step-by-step instructions, tips, and resources';

-- Update the Simple Linux Introduction lab with instructions
UPDATE lab_templates
SET instructions = '{
  "overview": "Welcome to your first Linux lab! In this exercise, you''ll learn the fundamental commands that every Linux user needs to know. By the end, you''ll be comfortable creating directories, files, and understanding how the Linux filesystem works.",
  "learning_objectives": [
    "Understand the Linux directory structure and home directory concept",
    "Create and organize directories using the mkdir command",
    "Create and manipulate files using touch and echo",
    "View filesystem information using ls and df commands",
    "Understand file paths (absolute vs relative)"
  ],
  "prerequisites": [
    "Basic computer literacy",
    "Access to a terminal (provided in this lab)"
  ],
  "steps": [
    {
      "id": "step-1",
      "title": "Understanding Your Home Directory",
      "objective_id": "create-directory",
      "content": "## What is a Home Directory?\n\nIn Linux, every user has a personal space called a **home directory**. This is where you store your personal files, configurations, and projects. It''s similar to \"My Documents\" in Windows or your user folder on a Mac.\n\nYour home directory path is `/home/student` (or `/home/<your-username>` on other systems). Linux provides a convenient shortcut: the tilde symbol `~` always refers to your home directory.\n\n## Why Organize with Directories?\n\nJust like physical folders help organize papers on your desk, directories (folders) in Linux help organize your files. Good organization makes it easier to:\n- Find files quickly\n- Back up important data\n- Share specific projects with others\n- Keep your system clean and manageable\n\n## Creating Your First Directory\n\nThe `mkdir` command (short for \"make directory\") creates new directories.\n\n**Syntax:** `mkdir [options] directory_name`\n\n### Your Task\n\nCreate a directory called `mywork` in your home directory:\n\n```bash\nmkdir ~/mywork\n```\n\n**Breaking it down:**\n- `mkdir` - the command to create a directory\n- `~/mywork` - the path where you want to create it\n  - `~` expands to `/home/student`\n  - So this creates `/home/student/mywork`\n\n### Verify It Worked\n\nAfter creating the directory, you can verify it exists:\n\n```bash\nls ~\n```\n\nYou should see `mywork` listed in the output."
    },
    {
      "id": "step-2",
      "title": "Creating an Empty File",
      "objective_id": "create-file",
      "content": "## What is the touch Command?\n\nThe `touch` command is one of the most versatile commands in Linux. Its primary purposes are:\n\n1. **Create empty files** - If a file doesn''t exist, `touch` creates it\n2. **Update timestamps** - If a file exists, `touch` updates its modification time\n\nIn this step, we''ll use it to create a new, empty file.\n\n## Creating notes.txt\n\n**Syntax:** `touch file_path`\n\n### Your Task\n\nCreate a file called `notes.txt` inside your `mywork` directory:\n\n```bash\ntouch ~/mywork/notes.txt\n```\n\n### Verify the File Exists\n\n```bash\nls ~/mywork\n```\n\nYou should see `notes.txt` in the output."
    },
    {
      "id": "step-3",
      "title": "Writing Content to a File",
      "objective_id": "write-content",
      "content": "## Output Redirection: The Power of >\n\nLinux has a powerful feature called **output redirection**. Normally, when you run a command, its output appears on your screen. The `>` operator redirects this output to a file instead.\n\n## The echo Command\n\n`echo` simply prints text to standard output. Combined with redirection, it becomes a quick way to write content to files.\n\n**Examples:**\n- `echo \"Hello\"` - prints \"Hello\" to screen\n- `echo \"Hello\" > file.txt` - writes \"Hello\" to file.txt\n\n## Overwrite vs Append\n\n| Operator | Behavior |\n|----------|----------|\n| `>` | **Overwrites** the file |\n| `>>` | **Appends** to the file |\n\n### Your Task\n\nWrite the text \"Hello, Kootenai!\" into your notes.txt file:\n\n```bash\necho \"Hello, Kootenai!\" > ~/mywork/notes.txt\n```\n\n### Verify the Content\n\n```bash\ncat ~/mywork/notes.txt\n```\n\nYou should see: `Hello, Kootenai!`"
    },
    {
      "id": "step-4",
      "title": "Checking Disk Space",
      "objective_id": "check-disk-space",
      "content": "## Why Monitor Disk Space?\n\nRunning out of disk space can cause serious problems:\n- Applications may crash\n- System logs can''t be written\n- Databases may become corrupted\n\n## The df Command\n\n`df` stands for \"disk free\" and shows how much space is available.\n\n**Common options:**\n- `df` - basic output in 1K blocks\n- `df -h` - **human-readable** format (KB, MB, GB)\n\n### Your Task\n\nCheck the available disk space:\n\n```bash\ndf -h\n```\n\n### Understanding the Output\n\n| Column | Meaning |\n|--------|---------|\n| Filesystem | The device name |\n| Size | Total capacity |\n| Used | Space currently used |\n| Avail | Free space remaining |\n| Use% | Percentage used |\n| Mounted on | Where it''s accessible |"
    },
    {
      "id": "step-5",
      "title": "Listing Directory Contents",
      "objective_id": "list-files",
      "content": "## The ls Command\n\n`ls` (list) is probably the most frequently used Linux command.\n\n## Common ls Options\n\n| Option | Purpose |\n|--------|---------|\n| `-l` | Long format (details) |\n| `-a` | Show hidden files |\n| `-h` | Human-readable sizes |\n\n**Pro tip:** Combine options! `ls -lah` shows all files in long, human-readable format.\n\n### Your Task\n\nList the contents of your mywork directory:\n\n```bash\nls ~/mywork\n```\n\nFor more details:\n```bash\nls -l ~/mywork\n```\n\nYou should see `notes.txt` with details about permissions, owner, size, and modification time."
    }
  ],
  "summary": "## Congratulations!\n\nYou''ve completed the Simple Linux Introduction lab. You''ve learned:\n\n1. **Home Directory (`~`)** - Your personal space in Linux\n2. **mkdir** - Create directories\n3. **touch** - Create empty files\n4. **echo with >** - Write content to files\n5. **df -h** - Check disk space\n6. **ls** - List directory contents\n\n## Next Steps\n\nNow that you understand these basics, you''re ready to learn about file permissions, text editors, and more advanced file operations!",
  "tips": [
    "Use Tab for auto-completion: type ''cd ~/myw'' and press Tab to complete to ''mywork''",
    "Use the up arrow key to recall previous commands",
    "The ''clear'' command cleans up your terminal screen",
    "Use ''man <command>'' to read the manual for any command"
  ],
  "resources": [
    {"title": "Linux Command Line Basics", "url": "https://ubuntu.com/tutorials/command-line-for-beginners"},
    {"title": "The Linux Filesystem Hierarchy", "url": "https://www.pathname.com/fhs/"}
  ]
}'::jsonb
WHERE name = 'Simple Linux Introduction';

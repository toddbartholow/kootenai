/**
 * Shared form option constants used across multiple views.
 * Centralizes dropdown options to eliminate duplication.
 */

export interface SelectOption<T = string> {
  label: string
  value: T
  description?: string
}

// Difficulty options (without "All" filter option)
export const difficultyOptions: SelectOption[] = [
  { label: 'Beginner', value: 'beginner' },
  { label: 'Intermediate', value: 'intermediate' },
  { label: 'Advanced', value: 'advanced' },
  { label: 'Expert', value: 'expert' },
] as const

// Difficulty filter options (includes "All" option for filtering)
export const difficultyFilterOptions: SelectOption<string | null>[] = [
  { label: 'All Difficulties', value: null },
  { label: 'Beginner', value: 'beginner' },
  { label: 'Intermediate', value: 'intermediate' },
  { label: 'Advanced', value: 'advanced' },
  { label: 'Expert', value: 'expert' },
] as const

// Platform options
export const platformOptions: SelectOption[] = [
  { label: 'Proxmox', value: 'proxmox' },
  { label: 'CloudStack', value: 'cloudstack' },
] as const

// Platform filter options (includes "All" option for filtering)
export const platformFilterOptions: SelectOption<string | null>[] = [
  { label: 'All Platforms', value: null },
  { label: 'Proxmox', value: 'proxmox' },
  { label: 'CloudStack', value: 'cloudstack' },
] as const

// Visibility options (full set for admins)
export const visibilityOptions: SelectOption[] = [
  { label: 'Global (Everyone)', value: 'global' },
  { label: 'Organization Only', value: 'organization' },
  { label: 'Private', value: 'private' },
] as const

// Visibility options (restricted for non-admins)
export const visibilityOptionsRestricted: SelectOption[] = [
  { label: 'Private', value: 'private' },
] as const

// VM Template options
export const templateOptions: SelectOption[] = [
  { label: 'Ubuntu 22.04 Server', value: 'ubuntu-22.04-server' },
  { label: 'Ubuntu 22.04 Desktop', value: 'ubuntu-22.04-desktop' },
  { label: 'Debian 12', value: 'debian-12' },
  { label: 'Rocky Linux 9', value: 'rocky-linux-9' },
  { label: 'Windows Server 2022', value: 'windows-server-2022' },
  { label: 'Kali Linux', value: 'kali-linux' },
] as const

// Trigger type options for objectives
export const triggerTypeOptions: SelectOption[] = [
  { label: 'File Exists', value: 'file_exists', description: 'Check if a file or directory exists' },
  { label: 'File Content', value: 'file_content', description: 'Check if a file contains specific text' },
  { label: 'Command Executed', value: 'command_executed', description: 'Check if a command was run' },
  { label: 'Service Running', value: 'service_running', description: 'Check if a service is active' },
] as const

// Question type options
export const questionTypeOptions: SelectOption[] = [
  { label: 'Text Answer', value: 'text' },
  { label: 'Multiple Choice', value: 'multiple_choice' },
] as const

// Validation type options for text questions
export const validationTypeOptions: SelectOption[] = [
  { label: 'Exact Match', value: 'exact' },
  { label: 'Regex Pattern', value: 'regex' },
] as const

// Unlock type options for pathway modules
export const unlockTypeOptions: SelectOption[] = [
  { label: 'Always Available', value: 'always' },
  { label: 'After Previous Module', value: 'sequential' },
  { label: 'After Specific Module', value: 'specific' },
  { label: 'After Date', value: 'date' },
] as const

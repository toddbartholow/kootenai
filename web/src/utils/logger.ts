/**
 * Production-ready logging utility for the Kootenai frontend.
 *
 * Features:
 * - Environment-aware: Suppresses debug/info logs in production
 * - Consistent formatting with timestamps and prefixes
 * - Structured logging support
 * - Can be extended with external logging services (Sentry, LogRocket, etc.)
 */

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface LogContext {
  /** Component or module name */
  module?: string
  /** Additional structured data */
  [key: string]: unknown
}

interface LoggerConfig {
  /** Minimum log level to output */
  level: LogLevel
  /** Whether logging is enabled */
  enabled: boolean
  /** Whether to include timestamps */
  timestamps: boolean
  /** Prefix for all log messages */
  prefix: string
}

const LOG_LEVELS: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
}

// Determine environment
const isDevelopment = import.meta.env.DEV
const isProduction = import.meta.env.PROD

// Default configuration based on environment
const defaultConfig: LoggerConfig = {
  level: isDevelopment ? 'debug' : 'warn',
  enabled: true,
  timestamps: isDevelopment,
  prefix: '[Kootenai]',
}

let config: LoggerConfig = { ...defaultConfig }

/**
 * Configure the logger
 */
export function configureLogger(options: Partial<LoggerConfig>): void {
  config = { ...config, ...options }
}

/**
 * Check if a log level should be output
 */
function shouldLog(level: LogLevel): boolean {
  if (!config.enabled) return false
  return LOG_LEVELS[level] >= LOG_LEVELS[config.level]
}

/**
 * Format a log message with optional context
 */
function formatMessage(level: LogLevel, message: string, context?: LogContext): string {
  const parts: string[] = []

  if (config.prefix) {
    parts.push(config.prefix)
  }

  if (config.timestamps) {
    parts.push(`[${new Date().toISOString()}]`)
  }

  parts.push(`[${level.toUpperCase()}]`)

  if (context?.module) {
    parts.push(`[${context.module}]`)
  }

  parts.push(message)

  return parts.join(' ')
}

/**
 * Extract error details for logging
 */
function formatError(error: unknown): object {
  if (error instanceof Error) {
    return {
      name: error.name,
      message: error.message,
      stack: isDevelopment ? error.stack : undefined,
    }
  }
  return { value: String(error) }
}

/**
 * Core logging function
 */
function log(level: LogLevel, message: string, context?: LogContext, ...args: unknown[]): void {
  if (!shouldLog(level)) return

  const formattedMessage = formatMessage(level, message, context)
  const consoleMethod = level === 'debug' ? 'log' : level

  // Extract extra context data (excluding module)
  const extraContext = context
    ? Object.entries(context)
        .filter(([key]) => key !== 'module')
        .reduce((acc, [key, val]) => ({ ...acc, [key]: val }), {})
    : null

  const hasExtraContext = extraContext && Object.keys(extraContext).length > 0

  if (args.length > 0 || hasExtraContext) {
    console[consoleMethod](formattedMessage, ...(hasExtraContext ? [extraContext] : []), ...args)
  } else {
    console[consoleMethod](formattedMessage)
  }

  // Hook for external logging services (e.g., Sentry, LogRocket)
  if (isProduction && (level === 'error' || level === 'warn')) {
    // Example: Send to external service
    // captureException(message, { level, context, args })
  }
}

/**
 * Main logger object
 */
export const logger = {
  /**
   * Debug-level logging (hidden in production)
   */
  debug(message: string, context?: LogContext, ...args: unknown[]): void {
    log('debug', message, context, ...args)
  },

  /**
   * Info-level logging (hidden in production)
   */
  info(message: string, context?: LogContext, ...args: unknown[]): void {
    log('info', message, context, ...args)
  },

  /**
   * Warning-level logging
   */
  warn(message: string, context?: LogContext, ...args: unknown[]): void {
    log('warn', message, context, ...args)
  },

  /**
   * Error-level logging
   */
  error(message: string, error?: unknown, context?: LogContext): void {
    const errorDetails = error ? formatError(error) : undefined
    log('error', message, context, errorDetails)
  },

  /**
   * Create a scoped logger for a specific module
   */
  scope(module: string): ScopedLogger {
    return new ScopedLogger(module)
  },

  /**
   * Configure the logger
   */
  configure: configureLogger,

  /**
   * Get current configuration
   */
  getConfig(): LoggerConfig {
    return { ...config }
  },
}

/**
 * Scoped logger for a specific module
 */
class ScopedLogger {
  constructor(private module: string) {}

  debug(message: string, context?: Omit<LogContext, 'module'>, ...args: unknown[]): void {
    log('debug', message, { ...context, module: this.module }, ...args)
  }

  info(message: string, context?: Omit<LogContext, 'module'>, ...args: unknown[]): void {
    log('info', message, { ...context, module: this.module }, ...args)
  }

  warn(message: string, context?: Omit<LogContext, 'module'>, ...args: unknown[]): void {
    log('warn', message, { ...context, module: this.module }, ...args)
  }

  error(message: string, error?: unknown, context?: Omit<LogContext, 'module'>): void {
    const errorDetails = error ? formatError(error) : undefined
    log('error', message, { ...context, module: this.module }, errorDetails)
  }
}

/**
 * Pre-configured scoped loggers for common modules
 */
export const loggers = {
  websocket: logger.scope('WebSocket'),
  auth: logger.scope('Auth'),
  router: logger.scope('Router'),
  api: logger.scope('API'),
  console: logger.scope('Console'),
  store: logger.scope('Store'),
  session: logger.scope('Session'),
}

export default logger

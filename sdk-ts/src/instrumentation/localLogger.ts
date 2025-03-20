/* eslint-disable no-console */
import pino from "pino";
import { Writable } from "stream";

const originalConsole = {
  info: console.info,
  log: console.log,
  error: console.error,
  warn: console.warn,
  debug: console.debug
} as const;

const formatLog = (level: number, message: string) => {
  const colors: Record<number, string> = {
    10: '\x1b[35m', // trace (magenta)
    20: '\x1b[36m', // debug (cyan)
    30: '\x1b[32m', // info (vert)
    40: '\x1b[33m', // warn (jaune)
    50: '\x1b[31m', // error (rouge)
    60: '\x1b[41m', // fatal (fond rouge)
  };
  const reset = '\x1b[0m';

  const levelName: Record<number, string> = {
    10: 'TRACE',
    20: 'DEBUG',
    30: 'INFO',
    40: 'WARN',
    50: 'ERROR',
    60: 'FATAL',
  };

  const color = colors[level] || '\x1b[37m'; // Blanc par défaut
  return `${color}[${levelName[level] || 'LOG'}]${reset} ${message}`;
};

const customStream = new Writable({
  write(chunk, encoding, callback) {
    try {
      const logMessage = JSON.parse(chunk.toString());
      const formattedMessage = formatLog(logMessage.level, logMessage.msg);

      // Mapping vers la bonne fonction originalConsole
      switch (logMessage.level) {
        case 10: // trace
        case 20: // debug
          originalConsole.debug(formattedMessage);
          break;
        case 30: // info
          originalConsole.info(formattedMessage);
          break;
        case 40: // warn
          originalConsole.warn(formattedMessage);
          break;
        case 50: // error
        case 60: // fatal
          originalConsole.error(formattedMessage);
          break;
        default:
          originalConsole.log(formattedMessage);
      }
    } catch (err) {
      originalConsole.error('Erreur de parsing des logs :', err);
    }
    callback();
  },
});

const loggerConfiguration = {
  level: process.env.BL_LOG_LEVEL || 'info',
};
const localLogger = pino(loggerConfiguration, customStream);

export default localLogger;

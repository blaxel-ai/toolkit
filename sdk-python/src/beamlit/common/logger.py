import logging


class ColoredFormatter(logging.Formatter):
    COLORS = {
        "DEBUG": "\033[1;36m",  # Cyan
        "INFO": "\033[1;32m",  # Green
        "WARNING": "\033[1;33m",  # Yellow
        "ERROR": "\033[1;31m",  # Red
        "CRITICAL": "\033[1;41m",  # Red background
    }

    def format(self, record):
        n_spaces = len("CRITICAL") - len(record.levelname)
        tab = " " * n_spaces
        color = self.COLORS.get(record.levelname, "\033[0m")
        record.levelname = f"{color}{record.levelname}\033[0m:{tab}"
        return super().format(record)


def init(log_level: str):
    logging.getLogger("uvicorn.access").handlers.clear()
    logging.getLogger("uvicorn.access").propagate = False
    logging.getLogger("uvicorn.error").handlers.clear()
    logging.getLogger("uvicorn.error").propagate = False
    logging.getLogger("httpx").handlers.clear()
    logging.getLogger("httpx").propagate = False

    handler = logging.StreamHandler()
    handler.setFormatter(ColoredFormatter("%(levelname)s %(name)s - %(message)s"))
    logging.basicConfig(level=log_level, handlers=[handler])

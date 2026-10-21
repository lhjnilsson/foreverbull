class ConnectionError(Exception):
    def __init__(self, *args: object, cause: str | None = None):
        super().__init__(*args)
        # The redacted "Type: message" of the error that caused this one.
        self.cause = cause


class DatabaseError(Exception):
    """The database could not be opened for a reason other than the connection."""

    def __init__(self, *args: object, cause: str | None = None):
        super().__init__(*args)
        self.cause = cause


class DatabaseDriverError(DatabaseError):
    """The Python driver for the database URL is not installed. The database is not at fault."""

    def __init__(self, *args: object, driver: str, cause: str | None = None):
        super().__init__(*args, cause=cause)
        self.driver = driver


class ConfigurationError(Exception):
    pass


class RuntimeError(Exception):
    pass

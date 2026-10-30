class PlainSentence(Exception):
    """A failure whose message is the whole sentence the user sees.

    The data service sends it as is and without details: the cause, when
    there is one, is logged where it is raised. A failed ingest sends it
    with ABORTED and a refused run with FAILED_PRECONDITION, and the broker
    shows it to `fbull` unchanged.
    """

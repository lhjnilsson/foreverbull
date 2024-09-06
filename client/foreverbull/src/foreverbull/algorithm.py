from foreverbull.worker import WorkerPool


def main():
    pool = WorkerPool("bad_file")
    with pool:
        pass

import signal
import socket
import sys
from multiprocessing import set_start_method

from foreverbull import Foreverbull, broker


def main():
    foreverbull = Foreverbull(file_path=sys.argv[1])
    with foreverbull as fb:
        broker.service.update_instance(socket.gethostname(), True)
        signal.signal(signal.SIGINT, lambda x, y: fb._stop_event.set())
        signal.signal(signal.SIGTERM, lambda x, y: fb._stop_event.set())
        fb.join()
        broker.service.update_instance(socket.gethostname(), False)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 foreverbull/_run_instance.py <file_path>")
        exit(1)
    set_start_method("spawn")
    main()

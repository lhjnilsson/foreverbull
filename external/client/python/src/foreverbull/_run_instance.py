import os
import signal
import socket

from foreverbull import Foreverbull, broker


def main():
    foreverbull = Foreverbull(file_path=os.argv[1])
    with foreverbull as fb:
        broker.service.update_instance(socket.gethostname(), True)
        signal.signal(signal.SIGINT, lambda x, y: foreverbull._stop_event.set())
        signal.signal(signal.SIGTERM, lambda x, y: foreverbull._stop_event.set())
        fb.join()
        broker.service.update_instance(socket.gethostname(), False)


if __name__ == "__main__":
    if len(os.argv) != 2:
        print("Usage: python3 foreverbull/_run_instance.py <file_path>")
        exit(1)
    main()

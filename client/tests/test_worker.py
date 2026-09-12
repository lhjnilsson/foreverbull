from multiprocessing import Event
from threading import Thread

import pytest

from foreverbull import exceptions
from foreverbull import worker
from foreverbull.pb.foreverbull.service.v1 import worker_pb2


class TestWorkerInstance:
    def test_configure_bad_file(self):
        w = worker.WorkerInstance("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(worker_pb2.ExecutionConfiguration())

    def test_configure_execution(self, fb_env, fb_database, parallel_algo_file, namespace_server):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        w.configure_execution(request)

    @pytest.mark.parametrize(
        "new_attr, exc_msg",
        [
            (["brokerPort", 1234], "Unable to connect to broker"),
            (["namespacePort", 1234], "Unable to connect to namespace"),
            (["databaseURL", "bad_url"], "Unable to connect to database"),
        ],
    )
    def test_configure_execution_error(self, fb_env, parallel_algo_file, namespace_server, new_attr, exc_msg):
        algorithm, request, _ = parallel_algo_file
        w = worker.WorkerInstance(algorithm._file_path)
        setattr(request, *new_attr)
        with pytest.raises(exceptions.ConfigurationError, match=exc_msg):
            w.configure_execution(request)

    def test_run_execution(self, fb_env, fb_database, namespace_server, non_parallel_algo_file):
        algorithm, request, process_symbols = non_parallel_algo_file
        event = Event()

        def process_and_stop():
            process_symbols()
            event.set()

        w = worker.WorkerInstance(algorithm._file_path)
        w.configure_execution(request)
        t = Thread(target=process_and_stop, args=())
        t.start()
        w.run_execution(
            event,
        )


class TestWorkerPool:
    def test_bad_algorithm_file_path(self):
        pool = worker.WorkerPool("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            with pool:
                pass

    def test_configure_not_running(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pytest.raises(RuntimeError):
            pool.configure_execution(worker_pb2.ExecutionConfiguration())

    def test_configure(self, fb_env, fb_database, namespace_server, parallel_algo_file):
        algorithm, request, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path, executors=1)
        with pool:
            pool.configure_execution(request)

    @pytest.mark.parametrize(
        "new_attr,exc_msg",
        [
            (["brokerPort", 1234], "Unable to connect to broker"),
            (["namespacePort", 1234], "Unable to connect to namespace"),
            (["databaseURL", "bad_url"], "Unable to connect to database"),
        ],
    )
    def test_configure_error(self, fb_env, namespace_server, parallel_algo_file, new_attr, exc_msg):
        algorithm, request, _ = parallel_algo_file
        setattr(request, *new_attr)
        pool = worker.WorkerPool(algorithm._file_path, executors=1)
        with pool:
            with pytest.raises(exceptions.ConfigurationError, match=exc_msg):
                pool.configure_execution(request)

    def test_run_not_running(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pytest.raises(RuntimeError):
            pool.run_execution(Event())

    def test_run_not_configured(self, parallel_algo_file):
        algorithm, _, _ = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            with pytest.raises(exceptions.ConfigurationError):
                pool.run_execution(Event())

    def test_run_stop_before_process(self, fb_env, fb_database, namespace_server, parallel_algo_file):
        algorithm, request, process_symbols = parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            pool.configure_execution(request)
            stop_event = Event()
            pool.run_execution(stop_event)
            stop_event.set()

    def test_run(self, fb_env, fb_database, namespace_server, non_parallel_algo_file):
        algorithm, request, process_symbols = non_parallel_algo_file
        pool = worker.WorkerPool(algorithm._file_path)
        with pool:
            stop_event = Event()
            pool.configure_execution(request)
            pool.run_execution(stop_event)
            orders = process_symbols()
            stop_event.set()
            assert orders

from multiprocessing import Event
from threading import Thread

import pytest
from foreverbull import exceptions, worker
from foreverbull.pb.service import service_pb2


class TestWorkerInstance:
    def test_configure_bad_file(self):
        w = worker.WorkerInstance("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(service_pb2.ConfigureExecutionRequest())

    def test_configure_bad_broker_port(self, parallel_algo_file):
        file_name, request, _ = parallel_algo_file
        w = worker.WorkerInstance(file_name)
        request.brokerPort = 1234
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_namespace_port(self, parallel_algo_file):
        file_name, request, _ = parallel_algo_file
        w = worker.WorkerInstance(file_name)
        request.namespacePort = 1234
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_database(self, parallel_algo_file):
        file_name, request, _ = parallel_algo_file
        w = worker.WorkerInstance(file_name)
        request.databaseURL = "bad_url"
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    def test_configure_bad_parameters(self, parallel_algo_file):
        file_name, request, _ = parallel_algo_file
        w = worker.WorkerInstance(file_name)
        request.databaseURL = "bad_url"
        with pytest.raises(exceptions.ConfigurationError):
            w.configure_execution(request)

    @pytest.mark.skip()
    def test_configure_and_run_execution(self, namespace_server, parallel_algo_file):
        file_name, request, process_symbols = parallel_algo_file
        w = worker.WorkerInstance(file_name)
        w.configure_execution(request)
        t = Thread(target=process_symbols, args=())
        t.start()
        w.run_execution(
            service_pb2.RunExecutionRequest(),
            Event(),
        )


class TestWorkerPool:
    def test_bad_file_name(self):
        pool = worker.WorkerPool("bad_file")
        with pytest.raises(exceptions.ConfigurationError):
            with pool:
                pass

    def test_configure_not_running(self, parallel_algo_file):
        file_name, _, _ = parallel_algo_file
        pool = worker.WorkerPool(file_name)
        with pytest.raises(RuntimeError):
            pool.configure_execution(service_pb2.ConfigureExecutionRequest())

    @pytest.mark.parametrize(
        "broker_port,namespace_port,database_url,expected_exception",
        [
            (1234, None, None, exceptions.ConfigurationError),
            (None, 1234, None, exceptions.ConfigurationError),
            (None, None, "bad_url", exceptions.ConfigurationError),
            (None, None, None, None),
        ],
    )
    def test_configure(
        self, namespace_server, parallel_algo_file, broker_port, namespace_port, database_url, expected_exception
    ):
        file_name, request, process_symbols = parallel_algo_file
        if broker_port:
            request.brokerPort = broker_port
        if namespace_port:
            request.namespacePort = namespace_port
        if database_url:
            request.databaseURL = database_url
        pool = worker.WorkerPool(file_name)
        with pool:
            if expected_exception:
                with pytest.raises(expected_exception):
                    pool.configure_execution(request)
            else:
                pool.configure_execution(request)

    def test_run_not_running(self, parallel_algo_file):
        file_name, _, _ = parallel_algo_file
        pool = worker.WorkerPool(file_name)
        with pytest.raises(RuntimeError):
            pool.run_execution(service_pb2.RunExecutionRequest(), Event())

    def test_run_not_configured(self, namespace_server, parallel_algo_file):
        file_name, request, _ = parallel_algo_file
        pool = worker.WorkerPool(file_name)
        with pool:
            with pytest.raises(exceptions.ConfigurationError):
                pool.run_execution(service_pb2.RunExecutionRequest(), Event())

    def test_run_stop_before_process(self, namespace_server, parallel_algo_file):
        file_name, request, process_symbols = parallel_algo_file
        pool = worker.WorkerPool(file_name)
        with pool:
            pool.configure_execution(request)
            stop_event = Event()
            pool.run_execution(service_pb2.RunExecutionRequest(), stop_event)
            stop_event.set()

    def test_run(self, namespace_server, parallel_algo_file):
        file_name, request, process_symbols = parallel_algo_file
        pool = worker.WorkerPool(file_name)
        with pool:
            stop_event = Event()
            pool.configure_execution(request)
            pool.run_execution(service_pb2.RunExecutionRequest(), stop_event)
            orders = process_symbols()
            stop_event.set()

import axios from "axios";

async function createBacktest(
  name,
  backtestService,
  workerService,
  calendar,
  startTime,
  endTime,
  benchmark,
  symbols
) {
  const response = await axios.post(
    "http://127.0.0.1:8080/backtest/api/backtests",
    {
      name: name,
      backtest_service: backtestService,
      worker_service: workerService,
      calendar: calendar,
      start_time: startTime,
      end_time: endTime,
      benchmark: benchmark,
      symbols: symbols,
    }
  );
  return response.data;
}

async function getBacktest(name) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/backtests/" + name
  );
  return response.data;
}

async function getBacktests() {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/backtests"
  );
  return response.data;
}

async function updateBacktest(
  name,
  backtestService,
  workerService,
  calendar,
  startTime,
  endTime,
  symbols,
  benchmark
) {
  const response = await axios.patch(
    "http://127.0.0.1:8080/backtest/api/backtests/" + name,
    {
      backtest_service: backtestService,
      worker_service: workerService,
      calendar: calendar,
      start_time: startTime,
      end_time: endTime,
      benchmark: benchmark,
      symbols: symbols,
    }
  );
  return response.data;
}

async function deleteBacktest(name) {
  const response = await axios.delete(
    "http://127.0.0.1:8080/backtest/api/backtests/" + name
  );
  return response.data;
}

async function createSession(backtest, executions) {
  const response = await axios.post(
    "http://127.0.0.1:8080/backtest/api/sessions",
    {
      backtest: backtest,
      executions: executions,
      source: "ui",
      source_key: "ui",
    }
  );
  return response.data;
}

async function getSession(sessionid) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/sessions/" + sessionid
  );
  return response.data;
}

async function getSessions(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/sessions",
    { params: { backtest: backtest } }
  );
  return response.data;
}

async function getSessionExecutions(sessionid) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions",
    { params: { session: sessionid } }
  );
  return response.data;
}

async function getExecutions(session) {
  var response;
  if (session) {
    response = await axios.get(
      "http://127.0.0.1:8080/backtest/api/backtests/executions",
      { params: { session: session } }
    );
  } else {
    response = await axios.get(
      "http://127.0.0.1:8080/backtest/api/backtests/executions"
    );
  }
  return response.data;
}

async function getExecution(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" + backtest
  );
  return response.data;
}

async function getOrders(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" + backtest + "/orders"
  );
  return response.data;
}

async function getPositions(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" + backtest + "/positions"
  );
  return response.data;
}

async function getPeriods(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" + backtest + "/periods"
  );
  return response.data;
}

async function getMetrics(backtest) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" +
      backtest +
      "/periods/metrics"
  );
  return response.data;
}

async function getMetric(backtest, metric) {
  const response = await axios.get(
    "http://127.0.0.1:8080/backtest/api/executions/" +
      backtest +
      "/periods/metrics/" +
      metric
  );
  return response.data;
}

export {
  createBacktest,
  getBacktest,
  getBacktests,
  updateBacktest,
  deleteBacktest,
  createSession,
  getSession,
  getSessions,
  getSessionExecutions,
  getExecutions,
  getExecution,
  getOrders,
  getPositions,
  getPeriods,
  getMetrics,
  getMetric,
};

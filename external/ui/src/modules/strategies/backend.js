import axios from "axios";

async function getStrategies() {
  const response = await axios.get(
    "http://127.0.0.1:8080/strategy/api/strategies"
  );
  return response.data;
}

async function getStrategy(name) {
  const response = await axios.get(
    "http://127.0.0.1:8080/strategy/api/strategies/" + name
  );
  return response.data;
}

async function addStrategy(name, backtest) {
  const response = await axios.post(
    "http://127.0.0.1:8080/strategy/api/strategies",
    {
      name: name,
      backtest: backtest,
    }
  );
  return response.data;
}

async function updateStrategy(name, backtest, schedule) {
  const response = await axios.patch(
    "http://127.0.0.1:8080/strategy/api/strategies/" + name,
    {
      backtest: backtest,
      schedule: schedule,
    }
  );
  return response.data;
}

export { getStrategies, getStrategy, addStrategy, updateStrategy };

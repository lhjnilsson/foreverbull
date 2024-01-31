import axios from "axios";

async function getServices() {
  const response = await axios.get(
    "http://127.0.0.1:8080/service/api/services"
  );
  return response.data;
}

async function getService(name) {
  const response = await axios.get(
    "http://127.0.0.1:8080/service/api/services/" + name
  );
  return response.data;
}

async function addService(name, image) {
  const response = await axios.post(
    "http://127.0.0.1:8080/service/api/services",
    {
      name: name,
      image: image,
    }
  );
  return response.data;
}

async function imageInfo(name) {
  const response = await axios.get(
    "http://127.0.0.1:8080/service/api/services/" + name + "/image"
  );
  return response.data;
}

async function updateImage(name) {
  const response = await axios.put(
    "http://127.0.0.1:8080/service/api/services/" + name + "/image"
  );
  return response.data;
}

async function ingestBacktest(name, start, end, calendar, symbols, benchmark) {
  console.log(
    "Ingesting backtest",
    name,
    start,
    end,
    calendar,
    symbols,
    benchmark
  );
  const response = await axios.put(
    "http://127.0.0.1:8080/service/api/services/" + name + "/backtest/ingest",
    {
      start_time: start,
      end_time: end,
      calendar: calendar,
      symbols: symbols,
      benchmark: benchmark,
    }
  );
  return response.data;
}

async function getInstances(serviceName, key, role) {
  var response = null;
  if (serviceName) {
    response = await axios.get(
      "http://127.0.0.1:8080/service/api/instances?service=" + serviceName
    );
  } else if (key && role) {
    response = await axios.get(
      "http://127.0.0.1:8080/service/api/instances?key=" + key + "&role=" + role
    );
  } else {
    return null;
  }
  return response.data;
}

export {
  getServices,
  getService,
  addService,
  imageInfo,
  updateImage,
  ingestBacktest,
  getInstances,
};

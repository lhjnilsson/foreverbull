import axios from "axios";

async function getAssets(symbols = []) {
  const response = await axios.get(
    "http://127.0.0.1:8080/finance/api/assets",
    symbols,
  );
  return response.data;
}

async function ingestAsset(symbol) {
  const response = await axios.put("http://127.0.0.1:8080/finance/api/assets", {
    symbols: [symbol],
  });
  return response.data;
}

async function ingestOHLC(symbols, start_time, end_time) {
  const response = await axios.put("http://127.0.0.1:8080/finance/api/ohlc", {
    symbols: symbols,
    start_time: start_time,
    end_time: end_time,
  });
  return response.data;
}

export { getAssets, ingestAsset, ingestOHLC };

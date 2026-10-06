from foreverbull_testing.session import Session

from foreverbull import Algorithm
from foreverbull import Function
from foreverbull.models.finance import Asset
from foreverbull.models.finance import Assets
from foreverbull.models.finance import Portfolio


def test_asset_reads_synthetic_bars(session: Session):
    times = session.constant("SYN", 5, 42.0)
    asset = next(iter(session.assets(["SYN"], times[-1])))

    assert type(asset) is Asset
    assert asset.symbol == "SYN"
    frame = asset.stock_data
    assert list(frame.columns) == ["symbol", "time", "high", "low", "open", "close", "volume"]
    assert [float(value) for value in frame["close"]] == [42.0] * 5
    assert [float(value) for value in frame["open"]] == [42.0] * 5
    assert [float(value) for value in frame["high"]] == [42.0] * 5
    assert [float(value) for value in frame["low"]] == [42.0] * 5
    assert [int(value) for value in frame["volume"]] == [1] * 5


def test_asset_stock_data_is_the_last_252_bars(session: Session):
    # 253 distinct closes. The worker query keeps the last 252 at or before as_of.
    times = session.trend("LOOK", 253, 1.0, 253.0)
    asset = next(iter(session.assets(["LOOK"], times[-1])))
    closes = [float(value) for value in asset.stock_data["close"]]

    assert type(asset) is Asset
    assert len(closes) == 252
    assert closes[0] == 2.0
    assert closes[-1] == 253.0


def test_asset_stock_data_stops_at_as_of(session: Session):
    times = session.trend("LATE", 4, 10.0, 40.0)
    asset = next(iter(session.assets(["LATE"], times[1])))
    closes = [float(value) for value in asset.stock_data["close"]]

    assert closes == [10.0, 20.0]


def test_assets_read_each_synthetic_symbol(session: Session):
    times = session.constant("AAA", 3, 11.0)
    session.constant("BBB", 3, 22.0)
    assets = session.assets(["AAA", "BBB"], times[-1])

    assert type(assets) is Assets
    frame = assets.stock_data
    assert assets.symbols == ["AAA", "BBB"]
    assert list(frame.index.names) == ["symbol", "time"]
    assert [float(value) for value in frame.xs("AAA", level="symbol")["close"]] == [11.0] * 3
    assert [float(value) for value in frame.xs("BBB", level="symbol")["close"]] == [22.0] * 3
    iterated = {asset.symbol: type(asset) for asset in assets}
    assert iterated == {"AAA": Asset, "BBB": Asset}


def test_worker_metrics_round_trip(session: Session, fb_env, namespace_server):
    times = session.constant("AAA", 1, 10.0)
    session.constant("BBB", 1, 12.0)
    assets = session.assets(["AAA", "BBB"], times[-1])
    assets.set_metrics("score", {"AAA": 1.0, "BBB": 2.0})

    assert assets.get_metrics("score").to_dict() == {"AAA": 1.0, "BBB": 2.0}
    assert namespace_server["score"] == {"AAA": 1.0, "BBB": 2.0}

    held = next(item for item in assets if item.symbol == "BBB")
    held.set_metric("flag", 1.0)
    missed = next(item for item in assets if item.symbol == "AAA")
    assert held.get_metric("flag") == 1.0
    assert missed.get_metric("flag") is None
    assert namespace_server["flag"] == {"BBB": 1.0}


def test_run_sees_the_synthetic_close(session: Session):
    def handle_data(asset: Asset, portfolio: Portfolio) -> None:
        portfolio.order_target(asset.symbol, int(float(asset.stock_data["close"].iloc[-1])))

    times = session.constant("RUN", 2, 8.0)
    orders = session.run(Algorithm(functions=[Function(callable=handle_data)]), times[-1], 100_000)

    assert len(orders) == 1
    assert orders[0].symbol == "RUN"
    assert orders[0].amount == 8

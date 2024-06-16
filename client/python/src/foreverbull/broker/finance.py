import requests

from foreverbull import entity

from .http import api_call


@api_call(response_model=entity.finance.Asset)
def get_assets() -> requests.Request:
    return requests.Request(
        method="GET",
        url="/finance/api/assets",
        params={},
    )

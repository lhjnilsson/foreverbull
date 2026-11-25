package usercopy_test

import (
	"errors"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/usercopy"
	"github.com/stretchr/testify/require"
)

func listed(symbol, place, currency string) usercopy.MarketSymbol {
	return usercopy.MarketSymbol{Symbol: symbol, Place: place, Currency: currency}
}

func currencyOnly(symbol, provider, currency string) usercopy.MarketSymbol {
	return usercopy.MarketSymbol{Symbol: symbol, Provider: provider, Currency: currency, CurrencyOnly: true}
}

func suffixOnly(symbol, place string) usercopy.MarketSymbol {
	return usercopy.MarketSymbol{Symbol: symbol, Place: place}
}

func unknownSuffix(symbol, suffix string) usercopy.MarketSymbol {
	return usercopy.MarketSymbol{Symbol: symbol, Suffix: suffix}
}

func TestGlobalMarketsCreateOneListedSymbol(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateOneCurrency(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		currencyOnly("XYZ", "yahoo", "CAD"),
	}, nil, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': yahoo prices XYZ in CAD, not USD. Global markets aren't supported yet. The backtest wasn't saved. Remove XYZ, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateOneKnownSuffix(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		suffixOnly("VOD.L", "London"),
	}, nil, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateOneUnknownSuffix(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		unknownSuffix("ABC.XX", "XX"),
	}, nil, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': ABC.XX is listed outside the US (the .XX suffix isn't a US exchange). Global markets aren't supported yet. The backtest wasn't saved. Remove ABC.XX, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateSeveralSymbols(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		currencyOnly("XYZ", "yahoo", "CAD"),
		listed("VOD.L", "London", "GBP"),
		listed("BMW.DE", "Xetra", "EUR"),
	}, nil, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': 3 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP), XYZ (yahoo prices it in CAD). Global markets aren't supported yet. The backtest wasn't saved. Remove them, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateBenchmarkAlone(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", nil, &bench, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateBenchmarkCurrency(t *testing.T) {
	bench := currencyOnly("XYZ", "yahoo", "CAD")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", nil, &bench, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': yahoo prices the benchmark XYZ in CAD, not USD. Global markets aren't supported yet. The backtest wasn't saved. Set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateSymbolAndBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, &bench, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). The benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove VOD.L and set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateSeveralAndBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		listed("BMW.DE", "Xetra", "EUR"),
		listed("VOD.L", "London", "GBP"),
		currencyOnly("XYZ", "yahoo", "CAD"),
	}, &bench, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': 3 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP), XYZ (yahoo prices it in CAD). The benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Remove them and set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateETFBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", nil, &bench, true, true))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
}

func TestGlobalMarketsCreateETFBenchmarkWithSymbols(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, &bench, true, true))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). The benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
}

func TestGlobalMarketsCreatePartlySaved(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, false, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete uk-2024` first. Remove VOD.L, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreatePartlySavedBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreate("uk-2024", nil, &bench, false, false))
	require.EqualError(t, err, "Couldn't create backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete uk-2024` first. Set `benchmark` in the backtest file to a US-listed symbol such as SPY, then run `fbull backtest create` again.")
}

func TestGlobalMarketsRunOneSymbol(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsRun("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil))
	require.EqualError(t, err, "Couldn't run backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. Nothing was run. Delete it with `fbull backtest delete uk-2024`, remove VOD.L from the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsRunSeveral(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsRun("uk-2024", []usercopy.MarketSymbol{
		listed("BMW.DE", "Xetra", "EUR"),
		listed("VOD.L", "London", "GBP"),
		currencyOnly("XYZ", "yahoo", "CAD"),
	}, nil))
	require.EqualError(t, err, "Couldn't run backtest 'uk-2024': 3 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP), XYZ (yahoo prices it in CAD). Global markets aren't supported yet. Nothing was run. Delete it with `fbull backtest delete uk-2024`, remove them from the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsRunBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsRun("uk-2024", nil, &bench))
	require.EqualError(t, err, "Couldn't run backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. Nothing was run. Delete it with `fbull backtest delete uk-2024`, set `benchmark` in the backtest file to a US-listed symbol such as SPY, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsRunSymbolAndBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsRun("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, &bench))
	require.EqualError(t, err, "Couldn't run backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). The benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. Nothing was run. Delete it with `fbull backtest delete uk-2024`, remove VOD.L and set `benchmark` to a US-listed symbol such as SPY in the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsTradingOneSymbol(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsTrading("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil))
	require.EqualError(t, err, "Couldn't start trading with backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. No orders were sent. Pick another backtest, or delete it with `fbull backtest delete uk-2024`, remove VOD.L from the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsTradingSeveral(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsTrading("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
		listed("BMW.DE", "Xetra", "EUR"),
	}, nil))
	require.EqualError(t, err, "Couldn't start trading with backtest 'uk-2024': 2 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP). Global markets aren't supported yet. No orders were sent. Pick another backtest, or delete it with `fbull backtest delete uk-2024`, remove them from the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsTradingBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsTrading("uk-2024", nil, &bench))
	require.EqualError(t, err, "Couldn't start trading with backtest 'uk-2024': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. No orders were sent. Pick another backtest, or delete it with `fbull backtest delete uk-2024`, set `benchmark` in the backtest file to a US-listed symbol such as SPY, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsDeployOneSymbol(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsDeploy("mean-rev", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}))
	require.EqualError(t, err, "Couldn't start deployment 'mean-rev': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. The deployment wasn't started. Remove VOD.L from --symbols, then run `fbull strategy deploy mean-rev` again with the same options.")
}

func TestGlobalMarketsDeploySeveral(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsDeploy("mean-rev", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
		listed("BMW.DE", "Xetra", "EUR"),
	}))
	require.EqualError(t, err, "Couldn't start deployment 'mean-rev': 2 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP). Global markets aren't supported yet. The deployment wasn't started. Remove them from --symbols, then run `fbull strategy deploy mean-rev` again with the same options.")
}

func TestGlobalMarketsTradingSymbolAndBenchmark(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsTrading("uk-2024", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, &bench))
	require.EqualError(t, err, "Couldn't start trading with backtest 'uk-2024': VOD.L is listed outside the US (London, GBP). The benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. No orders were sent. Pick another backtest, or delete it with `fbull backtest delete uk-2024`, remove VOD.L and set `benchmark` to a US-listed symbol such as SPY in the backtest file, then create it again with `fbull backtest create`.")
}

func TestGlobalMarketsDeployThreeSymbols(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsDeploy("mean-rev", []usercopy.MarketSymbol{
		currencyOnly("XYZ", "yahoo", "CAD"),
		listed("VOD.L", "London", "GBP"),
		listed("BMW.DE", "Xetra", "EUR"),
	}))
	require.EqualError(t, err, "Couldn't start deployment 'mean-rev': 3 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP), XYZ (yahoo prices it in CAD). Global markets aren't supported yet. The deployment wasn't started. Remove them from --symbols, then run `fbull strategy deploy mean-rev` again with the same options.")
}

func TestLookupCreateOneTwoAndThree(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.LookupCreate("uk-2024", "yahoo", []string{"VOD.L"}, true)),
		"Couldn't create backtest 'uk-2024': yahoo didn't respond when checking where VOD.L is listed. The backtest wasn't saved. Wait a few minutes, then run `fbull backtest create` again.")
	require.EqualError(t, errors.New(usercopy.LookupCreate("uk-2024", "yahoo", []string{"VOD.L", "BMW.DE"}, true)),
		"Couldn't create backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE and VOD.L are listed. The backtest wasn't saved. Wait a few minutes, then run `fbull backtest create` again.")
	require.EqualError(t, errors.New(usercopy.LookupCreate("uk-2024", "yahoo", []string{"XYZ", "VOD.L", "BMW.DE"}, true)),
		"Couldn't create backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE, VOD.L and XYZ are listed. The backtest wasn't saved. Wait a few minutes, then run `fbull backtest create` again.")
}

func TestLookupCreatePartlySaved(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.LookupCreate("uk-2024", "yahoo", []string{"VOD.L"}, false)),
		"Couldn't create backtest 'uk-2024': yahoo didn't respond when checking where VOD.L is listed. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete uk-2024` first. Wait a few minutes, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateMixedRemoved(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreateMixed("b", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, "yahoo", []string{"ABC"}, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'b': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The backtest wasn't saved. Remove VOD.L, wait a few minutes, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateMixedPartlySaved(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreateMixed("b", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, "yahoo", []string{"ABC"}, false, false))
	require.EqualError(t, err, "Couldn't create backtest 'b': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The backtest was partly saved and couldn't be removed. Run `fbull backtest delete b` first. Remove VOD.L, wait a few minutes, then run `fbull backtest create` again.")
}

func TestGlobalMarketsCreateMixedFixedBenchmark(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsCreateMixed("b", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, "yahoo", []string{"ABC"}, true, true))
	require.EqualError(t, err, "Couldn't create backtest 'b': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The backtest wasn't saved. Please report it at https://github.com/lhjnilsson/foreverbull/issues with this message.")
}

func TestGlobalMarketsCreateMixedBenchmarkOnly(t *testing.T) {
	bench := listed("^FTSE", "London", "GBP")
	err := errors.New(usercopy.GlobalMarketsCreateMixed("b", nil, &bench, "yahoo", []string{"ABC"}, true, false))
	require.EqualError(t, err, "Couldn't create backtest 'b': the benchmark ^FTSE is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The backtest wasn't saved. Set `benchmark` in the backtest file to a US-listed symbol such as SPY, wait a few minutes, then run `fbull backtest create` again.")
}

func TestLookupRunOneTwoAndThree(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.LookupRun("uk-2024", "yahoo", []string{"VOD.L"})),
		"Couldn't run backtest 'uk-2024': yahoo didn't respond when checking where VOD.L is listed. Nothing was run. Wait a few minutes, then run `fbull backtest run` again.")
	require.EqualError(t, errors.New(usercopy.LookupRun("uk-2024", "yahoo", []string{"VOD.L", "BMW.DE"})),
		"Couldn't run backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE and VOD.L are listed. Nothing was run. Wait a few minutes, then run `fbull backtest run` again.")
	require.EqualError(t, errors.New(usercopy.LookupRun("uk-2024", "yahoo", []string{"XYZ", "BMW.DE", "VOD.L"})),
		"Couldn't run backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE, VOD.L and XYZ are listed. Nothing was run. Wait a few minutes, then run `fbull backtest run` again.")
}

func TestGlobalMarketsRunMixed(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsRunMixed("b", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, "yahoo", []string{"ABC"}))
	require.EqualError(t, err, "Couldn't run backtest 'b': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. Nothing was run. Delete it with `fbull backtest delete b`, remove VOD.L from the backtest file, then create it again with `fbull backtest create`.")
}

func TestLookupTradingOneTwoAndThree(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.LookupTrading("uk-2024", "yahoo", []string{"VOD.L"})),
		"Couldn't start trading with backtest 'uk-2024': yahoo didn't respond when checking where VOD.L is listed. No orders were sent. Wait a few minutes, then run `fbull trading run` again.")
	require.EqualError(t, errors.New(usercopy.LookupTrading("uk-2024", "yahoo", []string{"BMW.DE", "VOD.L"})),
		"Couldn't start trading with backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE and VOD.L are listed. No orders were sent. Wait a few minutes, then run `fbull trading run` again.")
	require.EqualError(t, errors.New(usercopy.LookupTrading("uk-2024", "yahoo", []string{"XYZ", "VOD.L", "BMW.DE"})),
		"Couldn't start trading with backtest 'uk-2024': yahoo didn't respond when checking where BMW.DE, VOD.L and XYZ are listed. No orders were sent. Wait a few minutes, then run `fbull trading run` again.")
}

func TestGlobalMarketsTradingMixed(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsTradingMixed("b", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, nil, "yahoo", []string{"ABC"}))
	require.EqualError(t, err, "Couldn't start trading with backtest 'b': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. No orders were sent. Pick another backtest, or delete it with `fbull backtest delete b`, remove VOD.L from the backtest file, then create it again with `fbull backtest create`.")
}

func TestLookupDeployOneTwoAndThree(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.LookupDeploy("mean-rev", "yahoo", []string{"VOD.L"})),
		"Couldn't start deployment 'mean-rev': yahoo didn't respond when checking where VOD.L is listed. The deployment wasn't started. Wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
	require.EqualError(t, errors.New(usercopy.LookupDeploy("mean-rev", "yahoo", []string{"VOD.L", "BMW.DE"})),
		"Couldn't start deployment 'mean-rev': yahoo didn't respond when checking where BMW.DE and VOD.L are listed. The deployment wasn't started. Wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
	require.EqualError(t, errors.New(usercopy.LookupDeploy("mean-rev", "yahoo", []string{"XYZ", "BMW.DE", "VOD.L"})),
		"Couldn't start deployment 'mean-rev': yahoo didn't respond when checking where BMW.DE, VOD.L and XYZ are listed. The deployment wasn't started. Wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
}

func TestGlobalMarketsOutsideNamesWhoWasNotChecked(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.GlobalMarketsOutside([]string{"VOD.L"})),
		"VOD.L wasn't checked when this trading session started, so it can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes VOD.L.")
	require.EqualError(t, errors.New(usercopy.GlobalMarketsOutside([]string{"VOD.L", "BP.L"})),
		"VOD.L and BP.L weren't checked when this trading session started, so they can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes them.")
	require.EqualError(t, errors.New(usercopy.GlobalMarketsOutside([]string{"VOD.L", "BP.L", "SHEL.L"})),
		"VOD.L, BP.L and SHEL.L weren't checked when this trading session started, so they can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes them.")
	require.EqualError(t, errors.New(usercopy.GlobalMarketsOutside(nil)),
		"A symbol wasn't checked when this trading session started, so it can't be traded. No orders were sent. Stop trading, then run `fbull trading run` again with a backtest that includes every symbol your algo trades.")
}

func TestDeploymentProfileUnchecked(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.DeploymentProfileUnchecked("mean-rev", "alpaca-paper", true)),
		"Couldn't start deployment 'mean-rev': this broker has no trading profiles set up, so foreverbull can't check where the symbols for 'alpaca-paper' are listed. The deployment wasn't started. Set up the trading profile 'alpaca-paper', then run `fbull strategy deploy mean-rev` again with the same options.")
	require.EqualError(t, errors.New(usercopy.DeploymentProfileUnchecked("mean-rev", "alpaca-paper", false)),
		"Couldn't start deployment 'mean-rev': there's no trading profile named 'alpaca-paper', so foreverbull can't check where its symbols are listed. The deployment wasn't started. Check the name against the trading profiles set up for foreverbull, then run `fbull strategy deploy mean-rev` again with the same options.")
}

func TestDeploymentListingNotStored(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.DeploymentListingNotStored("mean-rev")),
		"Couldn't start deployment 'mean-rev': foreverbull couldn't save or read where its symbols are listed. The deployment wasn't started. Run `fbenv status` to check that the environment is running, then run `fbull strategy deploy mean-rev` again with the same options.")
	require.EqualError(t, errors.New(usercopy.DeploymentListingNotStored("")),
		"Couldn't start the deployment: foreverbull couldn't save or read where its symbols are listed. The deployment wasn't started. Run `fbenv status` to check that the environment is running, then run `fbull strategy deploy` again with the same options.")
}

func TestDeploymentListingUnexpectedEndsWithReportIt(t *testing.T) {
	require.EqualError(t, errors.New(usercopy.DeploymentListingUnexpected("mean-rev")),
		"Couldn't start deployment 'mean-rev': foreverbull couldn't tell where its symbols are listed. The deployment wasn't started. "+usercopy.ReportIt)
	require.EqualError(t, errors.New(usercopy.DeploymentListingUnexpected("")),
		"Couldn't start the deployment: foreverbull couldn't tell where its symbols are listed. The deployment wasn't started. "+usercopy.ReportIt)
}

func TestGlobalMarketsDeployMixed(t *testing.T) {
	err := errors.New(usercopy.GlobalMarketsDeployMixed("mean-rev", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
	}, "yahoo", []string{"ABC"}))
	require.EqualError(t, err, "Couldn't start deployment 'mean-rev': VOD.L is listed outside the US (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The deployment wasn't started. Remove VOD.L from --symbols, wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
	plural := errors.New(usercopy.GlobalMarketsDeployMixed("mean-rev", []usercopy.MarketSymbol{
		listed("VOD.L", "London", "GBP"),
		listed("BMW.DE", "Xetra", "EUR"),
	}, "yahoo", []string{"ABC"}))
	require.EqualError(t, plural, "Couldn't start deployment 'mean-rev': 2 symbols aren't US-listed in USD: BMW.DE (Xetra, EUR), VOD.L (London, GBP). Global markets aren't supported yet. yahoo didn't respond when checking where ABC is listed. The deployment wasn't started. Remove them from --symbols, wait a few minutes, then run `fbull strategy deploy mean-rev` again with the same options.")
}

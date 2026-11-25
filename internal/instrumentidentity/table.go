// Package instrumentidentity is the instrument_identity table.
// Finance and backtest both create this statement. Neither copies the other.
package instrumentidentity

// Table is one listing of an ISIN on a venue over a half-open window.
// isin is required. price_unit is null, the currency itself, or GBX when
// the currency is GBP. GBX is one hundredth of a pound.
const Table = `
CREATE TABLE IF NOT EXISTS instrument_identity (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	isin text NOT NULL,
	mic text,
	currency text,
	price_unit text,
	valid_from date NOT NULL,
	valid_to date,
	CONSTRAINT instrument_identity_window CHECK (valid_to IS NULL OR valid_from < valid_to),
	CONSTRAINT instrument_identity_price_unit CHECK (
		price_unit IS NULL
		OR (
			currency IS NOT NULL
			AND (
				price_unit = currency
				OR (currency = 'GBP' AND price_unit = 'GBX')
			)
		)
	),
	CONSTRAINT instrument_identity_no_overlap EXCLUDE USING gist (
		isin WITH =,
		(COALESCE(mic, '')) WITH =,
		daterange(valid_from, valid_to, '[)') WITH &&
	)
);
`

// Index looks up an ISIN's windows by the day they start.
const Index = `
CREATE INDEX IF NOT EXISTS instrument_identity_isin_valid_from_idx
	ON instrument_identity (isin, valid_from);
`

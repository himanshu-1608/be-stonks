package provider

// Exchange identifies a stock exchange.
type Exchange string

const ExchangeNSE Exchange = "NSE"

// Operator is a comparison used in an alert condition.
type Operator string

const OpGTE Operator = ">="

// Attribute is the price field an alert watches.
type Attribute string

const AttrLTP Attribute = "LTP"

// AlertSpec is a broker-neutral description of an alert to create.
type AlertSpec struct {
	Name      string
	Symbol    string // tradingsymbol, no exchange suffix (e.g. "TARIL")
	Exchange  Exchange
	Attribute Attribute
	Operator  Operator
	Value     float64
}

// Alert is a broker-neutral view of an existing alert.
type Alert struct {
	Name string
	UUID string
}

// Config carries the inputs a provider constructor needs.
// Kept minimal and broker-neutral to avoid import cycles with the config package.
type Config struct {
	APIKey    string
	APISecret string
	DataDir   string // base data dir; providers store under DataDir/<name>/
}

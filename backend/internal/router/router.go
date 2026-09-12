package router

type Strategy string

const (
	StrategyProxy    Strategy = "proxy"
	StrategyParallel Strategy = "parallel"
)

type Match struct {
	Platform       string
	Strategy       Strategy
	UpstreamBase   string
	TargetURL      string
	IsIndex        bool
	IsMetadata     bool
	SmallFileBoost bool
}

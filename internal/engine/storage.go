package engine

type PlayerStore interface {
	GetPlayerScore(name string) int
	RecordWin(name string) error
	GetLeague() League
}

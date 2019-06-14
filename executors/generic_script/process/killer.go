package process

type Killer interface {
	Kill()
	ForceKill()
}

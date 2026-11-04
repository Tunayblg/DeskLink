//go:build !windows

package media

type source struct{}

func newSource() source { return source{} }

func (source) read(map[string]string) (State, map[string][]byte) { return State{Volume: -1}, nil }

func (source) readMixer() Mixer { return Mixer{Volume: -1} }

func (source) do(command) {}

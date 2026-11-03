//go:build !windows

package media

type source struct{}

func newSource() source { return source{} }

func (source) read(string) (Info, []byte) { return Info{Volume: -1}, nil }

func (source) setVolume(int) {}

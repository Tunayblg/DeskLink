//go:build !windows

package stats

type cpuSampler struct{}

func (*cpuSampler) sample() CPU { return CPU{} }

type gpuSampler struct{}

func (*gpuSampler) sample() *GPU { return nil }

func sampleRAM() RAM { return RAM{} }

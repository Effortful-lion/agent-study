package main

import (
	"math"
	"sort"
	"sync"
	"time"
)

/*
1. P50（中位数延迟）
定义：请求延迟的中位数
描述：一半用户、普通用户

2. P95（%5延迟）
定义：
描述：%5用户（少数用户）的体验瓶颈
*/

// LatencySnapshot 延迟指标快照，存储统计后的分位数据
type LatencySnapshot struct {
	Samples int           // 总采样次数
	P50     time.Duration // 50分位延迟（中位数）
	P95     time.Duration // 95分位延迟
}

// LatencyMetrics 延迟采集器，并发安全记录所有耗时样本并计算分位值
type LatencyMetrics struct {
	mu      sync.Mutex      // 互斥锁，保证并发读写samples安全
	samples []time.Duration // 存储每一次请求的耗时样本
}

// NewLatencyMetrics 初始化延迟指标采集器
func NewLatencyMetrics() *LatencyMetrics {
	return &LatencyMetrics{}
}

// Record 记录单次请求延迟，并发安全
// latency: 单次耗时，负数直接丢弃无效数据
func (m *LatencyMetrics) Record(latency time.Duration) {
	if latency < 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, latency)
}

// Snapshot 生成当前延迟统计快照，计算P50/P95分位
// 内部拷贝样本切片，排序计算，不影响原始存储数据
func (m *LatencyMetrics) Snapshot() LatencySnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 无采样数据时返回空快照
	if len(m.samples) == 0 {
		return LatencySnapshot{}
	}

	// 拷贝一份独立样本，避免排序修改原切片
	samples := append([]time.Duration(nil), m.samples...)
	// 升序排序样本，用于分位计算
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	return LatencySnapshot{
		Samples: len(samples),
		P50:     percentile(samples, 0.50),
		P95:     percentile(samples, 0.95),
	}
}

// percentile 根据排序后的样本计算指定百分位耗时
// samples: 已升序排序的延迟切片
// p: 百分位系数，范围0~1（如0.5代表P50，0.95代表P95）
// return: 对应分位的延迟值
func percentile(samples []time.Duration, p float64) time.Duration {
	// 计算目标下标：向上取整后-1，匹配百分位标准计算逻辑
	index := int(math.Ceil(float64(len(samples))*p)) - 1
	// 下标边界修正，防止越界
	if index < 0 {
		index = 0
	}
	if index >= len(samples) {
		index = len(samples) - 1
	}
	return samples[index]
}

package registry

import "fmt"

// Cost represents a skill cost tier.
type Cost string

// Skill cost tiers, ordered cheapest to most expensive.
const (
	CostCheap    Cost = "cheap"
	CostModerate Cost = "moderate"
	CostHeavy    Cost = "heavy"
)

// costRanks maps cost tiers to sort-order ranks.
var costRanks = map[Cost]int{
	CostCheap:    0,
	CostModerate: 1,
	CostHeavy:    2,
}

// ParseCost validates and returns a Cost from a raw string.
func ParseCost(s string) (Cost, error) {
	c := Cost(s)
	if _, ok := costRanks[c]; !ok {
		return "", fmt.Errorf("invalid cost %q (valid: cheap, moderate, heavy)", s)
	}
	return c, nil
}

// Rank returns the numeric sort order for this cost tier.
// Unknown costs sort last.
func (c Cost) Rank() int {
	if r, ok := costRanks[c]; ok {
		return r
	}
	return 99
}

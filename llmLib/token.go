package llmlib

func estimateTokens(s string) int {
	ascii, cjk := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			cjk++
		}
	}
	return ascii/4 + cjk*2/3 + 1
}

type Budget struct {
	Total        int
	SystemPrompt int
	Tools        int
	History      int
	Retrieved    int
}

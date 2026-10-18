// Package namegen generates human-readable names for pods and VMs.
// Names follow the adjective-verb-noun pattern like "cryptic-waddling-rain".
package namegen

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Word lists for name generation
var (
	adjectives = []string{
		"amber", "ancient", "autumn", "azure", "bold", "bright", "bronze",
		"calm", "celestial", "cobalt", "coral", "crimson", "crystal", "curious",
		"dancing", "daring", "dawn", "deep", "divine", "dusk", "dusty",
		"eager", "ebony", "emerald", "ethereal", "evening",
		"fading", "fierce", "fiery", "floating", "frozen", "gentle", "gilded",
		"glowing", "golden", "graceful", "granite", "hidden", "hollow", "humble",
		"ivory", "jade", "keen", "lapis", "lasting", "lavender", "lemon",
		"little", "lofty", "lonely", "lucky", "lunar", "mauve", "meadow",
		"midnight", "misty", "mossy", "noble", "obsidian", "ocean", "olive",
		"opal", "orange", "orchid", "patient", "peaceful", "pearl", "pine",
		"polar", "polished", "pristine", "proud", "purple", "quiet", "radiant",
		"rapid", "raven", "roaming", "rocky", "royal", "ruby", "rustic",
		"sacred", "sage", "sapphire", "scarlet", "serene", "shadow", "shining",
		"silent", "silver", "sleepy", "snowy", "solar", "spring", "steady",
		"stellar", "still", "summer", "sunny", "swift", "tawny", "tender",
		"tranquil", "twilight", "verdant", "violet", "warm", "wild", "winter",
		"wise", "wistful", "young", "zealous", "zesty",
	}

	verbs = []string{
		"blazing", "bounding", "breaking", "breathing", "bubbling",
		"calling", "chasing", "climbing", "coasting", "coursing", "cresting",
		"dashing", "diving", "drifting", "echoing", "fading", "falling",
		"floating", "flowing", "flying", "gazing", "gleaming", "gliding",
		"growing", "hiding", "howling", "humming", "jumping", "landing",
		"laughing", "leaping", "lingering", "looking", "looping", "meandering",
		"melting", "moving", "nesting", "passing", "pulsing", "racing",
		"raining", "reaching", "resting", "riding", "rising", "roaming",
		"rolling", "running", "rushing", "sailing", "searching", "seeking",
		"shining", "singing", "skating", "sleeping", "sliding", "soaring",
		"speaking", "spinning", "splashing", "standing", "streaming", "striding",
		"swaying", "sweeping", "swimming", "swinging", "swirling", "tickling",
		"tracing", "tumbling", "turning", "twisting", "wading",
		"waiting", "waking", "walking", "wandering", "watching", "waving",
		"weaving", "whirling", "whispering", "winding", "wishing",
	}

	nouns = []string{
		"aurora", "autumn", "bay", "beacon", "birch", "blossom", "boulder",
		"breeze", "brook", "canyon", "cascade", "cave", "cedar", "cliff",
		"cloud", "coast", "comet", "coral", "cosmos", "cove", "creek",
		"crescent", "crest", "crystal", "current", "dawn", "delta", "desert",
		"dew", "dove", "dream", "dune", "dusk", "eagle", "eclipse",
		"elm", "ember", "falcon", "fern", "field", "fire", "flame",
		"flower", "fog", "forest", "frost", "galaxy", "garden", "glacier",
		"glade", "glen", "grove", "harbor", "hawk", "haze", "heath",
		"hill", "horizon", "island", "ivy", "lake", "leaf", "light",
		"lily", "lotus", "maple", "marsh", "meadow", "mist", "moon",
		"moss", "mountain", "nebula", "night", "oak", "ocean", "orchid",
		"peak", "pebble", "pine", "plain", "planet", "pond", "prairie",
		"quartz", "rain", "rapids", "reef", "ridge", "river", "rose",
		"sage", "sea", "shade", "shadow", "shore", "sky", "snow",
		"spark", "spring", "star", "stone", "storm", "stream", "summit",
		"sun", "thunder", "tide", "trail", "tree", "valley", "violet",
		"wave", "whisper", "willow", "wind", "winter", "wolf", "wood",
	}
)

// Generate creates a new human-readable name in the format adjective-verb-noun.
// Examples: "cryptic-waddling-rain", "amber-floating-moon", "bold-dancing-river"
func Generate() string {
	adj := randomChoice(adjectives)
	verb := randomChoice(verbs)
	noun := randomChoice(nouns)
	return fmt.Sprintf("%s-%s-%s", adj, verb, noun)
}

// generateWithSuffix creates a name with an appended suffix for uniqueness.
// Example: "cryptic-waddling-rain-7x2k"
func generateWithSuffix(suffix string) string {
	return fmt.Sprintf("%s-%s", Generate(), suffix)
}

// generateUnique creates a name with a random 4-character suffix for guaranteed uniqueness.
// Example: "amber-floating-moon-a3f2"
func generateUnique() string {
	suffix := randomSuffix(4)
	return fmt.Sprintf("%s-%s", Generate(), suffix)
}

// randomChoice returns a random element from the given slice.
func randomChoice(choices []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
	if err != nil {
		// Fallback to first element if crypto/rand fails (should never happen)
		return choices[0]
	}
	return choices[n.Int64()]
}

// randomSuffix generates a random alphanumeric string of the specified length.
func randomSuffix(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var sb strings.Builder
	sb.Grow(length)
	for range length {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			sb.WriteByte('x')
			continue
		}
		sb.WriteByte(alphabet[n.Int64()])
	}
	return sb.String()
}

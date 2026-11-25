package launcher

// Session is what one OpenCode run is configured with.
type Session struct {
	Model        string            // the qualified model, "<provider id>/<model id>"
	Node         string            // the managed Node.js executable
	REAScript    string            // REA's entry script
	REAEnv       map[string]string // the environment REA's server is started with
	Instructions string            // the AGENTS.md OpenCode is given
}

// object is a JSON object that keeps its members in the order they were added.
type object []member

type member struct {
	key   string
	value any
}

// OpenCodeConfig returns the OpenCode configuration for s.
func OpenCodeConfig(s Session) object {
	return object{}
}

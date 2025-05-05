package skill

import (
	skill "yafai-skill/proto"
)

// APISpec represents the top-level structure of your YAML file.
type APISpec struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Actions     map[string]*Action `yaml:"actions"`
}

type SkillServer struct {
	Name                                  string
	Description                           string
	skill.UnimplementedSkillServiceServer                    // Embed the generated gRPC server interface          // Holds the parsed API specification from the YAML
	ActionsMap                            map[string]*Action // Optional: For quicker lookup of actions by name

}

// Action represents a single API action.
type Action struct {
	Name             string            `yaml:"-"`
	Desc             string            `yaml:"desc"`
	BaseURL          string            `yaml:"base_url"`
	Method           string            `yaml:"method"`
	Params           []Param           `yaml:"params"`
	Headers          map[string]string `yaml:"headers"`
	ResponseTemplate ResponseTemplate  `yaml:"response_template"`
}

type ResponseTemplate struct {
	Success string `yaml:"success"`
	Failure string `yaml:"failure"`
}

// Param represents a parameter for an API action.
type Param struct {
	Name     string `yaml:"name"` // Key in the params map
	Type     string `yaml:"type"`
	In       string `yaml:"in"`
	Desc     string `yaml:"desc"`
	Required bool   `yaml:"required"`
}

// RunningAction holds the information needed to execute an API call.
type RunningAction struct {
	Name             string
	Desc             string
	BaseURL          string
	Method           string
	Headers          map[string]string
	QueryParams      map[string]interface{}
	BodyParams       map[string]interface{} // To hold structured body parameters
	PathParams       map[string]interface{}
	Body             string           // For cases where the body needs to be a raw string (e.g., non-JSON)
	ResponseTemplate ResponseTemplate `yaml:"response_template"`
}

type ActionResult struct {
	Result string
	Error  error
}

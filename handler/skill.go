package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	pb "yafai-skill/proto"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

// Struct to handle YAML unmarshal to Go objects
func structToMap(m *structpb.Struct) map[string]interface{} {
	result := make(map[string]interface{})
	if m == nil {
		return result
	}
	for k, v := range m.Fields {
		result[k] = structpbValueToInterface(v)
	}
	return result
}

// Helper to convert structpb.Value to Go interface{}
func structpbValueToInterface(v *structpb.Value) interface{} {
	switch kind := v.Kind.(type) {
	case *structpb.Value_StringValue:
		return kind.StringValue
	case *structpb.Value_NumberValue:
		return kind.NumberValue
	case *structpb.Value_BoolValue:
		return kind.BoolValue
	case *structpb.Value_ListValue:
		arr := make([]interface{}, len(kind.ListValue.Values))
		for i, elem := range kind.ListValue.Values {
			arr[i] = structpbValueToInterface(elem)
		}
		return arr
	case *structpb.Value_StructValue:
		m := make(map[string]interface{})
		for k, v := range kind.StructValue.Fields {
			m[k] = structpbValueToInterface(v)
		}
		return m
	case *structpb.Value_NullValue:
		return nil
	default:
		return nil
	}
}

// GetActions RPC implementation
func (s *SkillServer) GetActions(ctx context.Context, req *pb.GetActionRequest) (res *pb.GetActionsResponse, err error) {
	slog.Info("Received GetActions request for task: %s", req.Task)

	// Check if ActionsMap is populated correctly
	toolDefinitions := s.ActionsMap // Access the parsed actions
	if len(toolDefinitions) == 0 {
		slog.Error("No actions found in ActionsMap")
	}

	actions := make([]*pb.Action, 0, len(toolDefinitions))
	for actionName, actionDef := range toolDefinitions {
		// Log the actionName and the action definition for debugging
		slog.Info("Processing action: %s", actionName)

		params := make([]*pb.Parameter, len(actionDef.Params))
		for i, p := range actionDef.Params {
			// Log each parameter being processed
			slog.Info("Processing param: %s", p.Name)

			params[i] = &pb.Parameter{
				Name:        p.Name,
				Type:        p.Type,
				In:          p.In,
				Description: p.Desc,
				Required:    p.Required,
				Enum:        p.Enum,
				Properties:  convertToPBParameters(p.Properties), // Convert nested properties
				Items:       convertToPBParameters(p.Items),      // Convert array items
			}
		}

		pbAction := &pb.Action{
			Name:        actionName,
			Description: actionDef.Desc,
			Method:      actionDef.Method,
			BaseUrl:     actionDef.BaseURL,
			Path:        "", // You might need to derive the specific path if it's part of the BaseURL with placeholders
			Params:      params,
			Headers:     actionDef.Headers,
		}

		actions = append(actions, pbAction)
	}

	res = &pb.GetActionsResponse{
		Actions: actions,
	}

	// Log the final response for debugging
	slog.Info("Returning response with %d actions", len(actions))

	return res, nil
}

// Helper function for nested properties conversion
func convertToPBParameters(params []*Param) []*pb.Parameter {
	if params == nil {
		return nil
	}

	pbParams := make([]*pb.Parameter, len(params))
	for i, p := range params {
		pbParams[i] = &pb.Parameter{
			Name:        p.Name,
			Type:        p.Type,
			In:          p.In,
			Description: p.Desc,
			Required:    p.Required,
			Enum:        p.Enum,
			Properties:  convertToPBParameters(p.Properties), // Recursively convert nested properties
		}

		// Handle array of objects, items could be objects with nested properties
		if p.Type == "array" && len(p.Items) > 0 {
			pbParams[i].Items = convertToPBParameters(p.Items) // Recursive for array of objects
		}
	}
	return pbParams
}

func (s *SkillServer) ExecuteAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	//slog.Info("%+v", req)
	reqID := uuid.New().String()
	slog.Info("ExecuteAction called | ID: %s | Action: %s | Time: %s", reqID, req.Name, time.Now().Format(time.RFC3339Nano))

	actionDef, ok := s.ActionsMap[req.Name]
	if !ok {
		return nil, fmt.Errorf("action '%s' not found", req.Name)
	}

	runningAction := RunningAction{
		Name:             req.Name,
		Desc:             actionDef.Desc,
		BaseURL:          actionDef.BaseURL,
		Method:           actionDef.Method,
		Headers:          actionDef.Headers,
		QueryParams:      make(map[string]interface{}),
		BodyParams:       make(map[string]interface{}),
		PathParams:       make(map[string]interface{}),
		ResponseTemplate: actionDef.ResponseTemplate,
	}

	// Convert queryParams, bodyParams, and pathParams from Struct to map
	runningAction.QueryParams = structToMap(req.QueryParams)
	runningAction.BodyParams = structToMap(req.BodyParams)
	runningAction.PathParams = structToMap(req.PathParams)

	// Set up parameters
	for _, paramDef := range actionDef.Params {
		var paramValue interface{}

		var ok bool

		switch strings.ToLower(paramDef.In) {
		case "query":
			paramValue, ok = runningAction.QueryParams[paramDef.Name]
			if ok {
				runningAction.QueryParams[paramDef.Name] = paramValue
			} else if paramDef.Required {
				return nil, fmt.Errorf("missing required query param '%s'", paramDef.Name)
			}
		case "path":
			paramValue, ok = runningAction.PathParams[paramDef.Name]
			if ok {
				runningAction.PathParams[paramDef.Name] = paramValue
			} else if paramDef.Required {
				return nil, fmt.Errorf("missing required path param '%s'", paramDef.Name)
			}
		case "body":
			if paramDef.In == "body" && paramDef.RootBody {
				raw, ok := runningAction.BodyParams[paramDef.Name].([]interface{})
				if !ok {
					return nil, fmt.Errorf("expected array for %s", paramDef.Name)
				}
				runningAction.RawBody = raw                     // store the slice
				delete(runningAction.BodyParams, paramDef.Name) // don’t send it in BodyParams
				break
			}
			paramValue, ok = runningAction.BodyParams[paramDef.Name]
			if ok {
				runningAction.BodyParams[paramDef.Name] = paramValue
			} else if paramDef.Required {
				return nil, fmt.Errorf("missing required body param '%s'", paramDef.Name)
			}
		}
	}

	// Execute action in background with context awareness
	resultChan := make(chan ActionResult, 1)
	slog.Info("Log Query Params : %+v", runningAction.QueryParams)
	slog.Info("Log Body Params : %+v", runningAction.BodyParams)
	slog.Info("Log Path Params : %+v", runningAction.PathParams)
	go func() {
		runningAction.Execute(ctx, resultChan) // Pass the incoming context
	}()

	var res ActionResult
	select {
	case res = <-resultChan:
		// Action completed (successfully or with error)
	case <-ctx.Done():
		slog.Info("ExecuteAction cancelled: %v", ctx.Err())
		return nil, ctx.Err()
	}

	if res.Error != nil {
		failTmpl, err := template.New("fail").Parse(runningAction.ResponseTemplate.Failure)
		if err != nil {
			slog.Error("Template parse error: %v", err)
			return nil, res.Error
		}
		var out bytes.Buffer
		_ = failTmpl.Execute(&out, map[string]string{"Error": res.Error.Error()})
		return &pb.ExecuteActionResponse{Response: out.String()}, res.Error
	}

	unquoted, err := strconv.Unquote(res.Result)
	if err != nil {
		slog.Info("Unquote failed: %v", err)
		unquoted = res.Result
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(unquoted), &data); err != nil {
		slog.Warn("JSON unmarshal failed: %v, using raw result", err)
		data = map[string]interface{}{"result": unquoted}
	}

	successTmpl, err := template.New("success").Parse(actionDef.ResponseTemplate.Success)
	if err != nil {
		slog.Error("Success template parse error: %v", err)
		return &pb.ExecuteActionResponse{Response: res.Result}, err // Fallback
	}
	var output bytes.Buffer
	if err := successTmpl.Execute(&output, data); err != nil {
		slog.Error("Success template execution error: %v", err)
		return &pb.ExecuteActionResponse{Response: res.Result}, err // Fallback
	}

	return &pb.ExecuteActionResponse{Response: output.String()}, nil
}

func (a *RunningAction) Execute(ctx context.Context, resultChan chan<- ActionResult) {
	u := a.BaseURL

	// Replace path parameters
	for key, value := range a.PathParams {
		placeholder := fmt.Sprintf("{%s}", key)
		u = strings.ReplaceAll(u, placeholder, fmt.Sprintf("%v", value)) // Use fmt.Sprintf to safely convert to string
	}

	// Query params
	query := url.Values{}
	for key, value := range a.QueryParams {
		// Handle different types for query params
		switch v := value.(type) {
		case string:
			query.Add(key, v)
		case bool:
			query.Add(key, fmt.Sprintf("%v", v))
		case float64:
			query.Add(key, fmt.Sprintf("%v", v))
		case []interface{}:
			// If the value is an array, we join elements as strings
			for _, item := range v {
				query.Add(key, fmt.Sprintf("%v", item))
			}
		default:
			query.Add(key, fmt.Sprintf("%v", v))
		}
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var payload io.Reader

	if a.RawBody != nil {
		// directly encode the slice or map
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(a.RawBody); err != nil {
			resultChan <- ActionResult{Error: err}
			return
		}
		payload = buf

	} else if len(a.BodyParams) > 0 {
		// fallback: encode the remaining map as JSON object
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(a.BodyParams); err != nil {
			resultChan <- ActionResult{Error: err}
			return
		}
		payload = buf
	}

	// Handle body params
	if len(a.BodyParams) > 0 {
		bodyBytes, err := json.Marshal(a.BodyParams)
		if err != nil {
			resultChan <- ActionResult{Error: err}
			return
		}
		payload = bytes.NewBuffer(bodyBytes)
	} else if a.Body != "" {
		payload = bytes.NewBuffer([]byte(a.Body))
	}

	slog.Info("Payload : %+v", payload)
	req, err := http.NewRequestWithContext(ctx, a.Method, u, payload)
	if err != nil {
		resultChan <- ActionResult{Error: err}
		return
	}

	// Set headers
	for key, value := range a.Headers {
		req.Header.Set(key, value)
	}
	auth_key := fmt.Sprintf("Bearer %s", os.Getenv("SKILL_KEY"))
	req.Header.Set("Authorization", auth_key)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		resultChan <- ActionResult{Error: err}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resultChan <- ActionResult{Error: err}
		return
	}
	slog.Info("Response Body: %s", string(body))
	if resp.StatusCode >= http.StatusBadRequest {
		err := fmt.Errorf("HTTP error: %s, body: %s", resp.Status, string(body))
		resultChan <- ActionResult{Error: err}
		return
	}

	resultChan <- ActionResult{Result: string(body)}
}

package cli

import (
	"fmt"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/detect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// selectRecipe resolves the recipe id for bob new and bob init: an explicit
// --recipe value when given, otherwise the recipe matching the detected stack,
// otherwise the historical go-agent-tool default for greenfield (missing or
// empty) and unrecognized repositories.
func selectRecipe(explicit string, detection detect.Result) (string, error) {
	if explicit != "" {
		if _, err := recipe.Version(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	if !detection.Detected() {
		return manifest.RecipeGoAgentTool, nil
	}
	// Honor agent-tool hint: existing bob.yaml recipe:go-agent-tool or Cobra layout.
	if detection.Primary == "go" && detection.KindHint == "agent-tool" {
		return manifest.RecipeGoAgentTool, nil
	}
	if id, ok := recipe.ForStack(detection.Primary); ok {
		return id, nil
	}
	return "", fmt.Errorf("no built-in recipe matches the detected stack %s; available recipes: %s (rerun with --recipe <id>)", detection.Describe(), strings.Join(recipe.IDs(), ", "))
}

// detectedRuntimeKind returns the detected kind hint when the recipe's runtime
// supports it, otherwise an empty kind so the recipe default applies.
func detectedRuntimeKind(recipeID string, detection detect.Result) string {
	runtime, ok := manifest.StackRecipeRuntime(recipeID)
	if !ok {
		return ""
	}
	for _, candidate := range runtime.Kinds {
		if candidate == detection.KindHint {
			return detection.KindHint
		}
	}
	return ""
}

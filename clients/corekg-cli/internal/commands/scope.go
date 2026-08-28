package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/insmtx/corekg/clients/corekg-cli/internal/api"
	"github.com/insmtx/corekg/clients/corekg-cli/internal/clierr"
)

func (a *app) resolveKnowledgeBase(ctx context.Context, active *activeProfile, selector string) (api.Forest, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		selector = strings.TrimSpace(active.Definition.KnowledgeBaseID)
	}
	if selector == "" {
		return api.Forest{}, clierr.Usage("kb_required", "no knowledge base selected; run `corekg-cli kb use ID_OR_NAME` or pass `--kb`")
	}
	selectorID, selectorIDErr := strconv.ParseUint(selector, 10, strconv.IntSize)
	if selectorIDErr == nil {
		if selectorID == 0 {
			return api.Forest{}, clierr.Usage("invalid_kb", "knowledge base ID must be positive")
		}
		var page api.ForestPage
		if err := active.Client.DoJSON(ctx, active.Credential.APIKey, "keapi.BatchGetForest", map[string]any{
			"forest_ids": []uint{uint(selectorID)},
		}, &page); err != nil {
			return api.Forest{}, clierr.Wrap("kb_lookup_failed", err)
		}
		if len(page.Data) == 0 {
			return api.Forest{}, clierr.New("kb_not_found", fmt.Sprintf("knowledge base %q does not exist or is not accessible", selector))
		}
		return page.Data[0], nil
	}

	forests, err := a.listAllForests(ctx, active)
	if err != nil {
		return api.Forest{}, err
	}
	matches := make([]api.Forest, 0, 1)
	for _, forest := range forests {
		if forest.Name == selector {
			matches = append(matches, forest)
		}
	}
	if len(matches) == 0 {
		return api.Forest{}, clierr.New("kb_not_found", fmt.Sprintf("knowledge base %q does not exist or is not accessible", selector))
	}
	if len(matches) > 1 {
		return api.Forest{}, clierr.New("kb_name_ambiguous", fmt.Sprintf("knowledge base name %q matches multiple records; use its ID", selector))
	}
	return matches[0], nil
}

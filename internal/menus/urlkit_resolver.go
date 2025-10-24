package menus

import (
	"context"
	"fmt"
	"strings"
	"sync"

	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
)

// URLKitResolverOptions configures the go-urlkit backed resolver.
type URLKitResolverOptions struct {
	Manager       *urlkit.RouteManager
	DefaultGroup  string
	LocaleGroups  map[string]string
	DefaultRoute  string
	SlugParam     string
	LocaleParam   string
	LocaleIDParam string
	RouteField    string
	ParamsField   string
	QueryField    string
}

// URLKitResolver resolves menu URLs using a go-urlkit RouteManager.
type URLKitResolver struct {
	manager *urlkit.RouteManager

	defaultGroup string
	localeGroups map[string]string

	defaultRoute  string
	slugParam     string
	localeParam   string
	localeIDParam string
	routeField    string
	paramsField   string
	queryField    string

	groupCache map[string]*urlkit.Group
	mu         sync.RWMutex
}

// NewURLKitResolver constructs a resolver backed by go-urlkit.
func NewURLKitResolver(opts URLKitResolverOptions) *URLKitResolver {
	if opts.SlugParam == "" {
		opts.SlugParam = "slug"
	}
	if opts.RouteField == "" {
		opts.RouteField = "route"
	}
	if opts.ParamsField == "" {
		opts.ParamsField = "params"
	}
	if opts.QueryField == "" {
		opts.QueryField = "query"
	}

	return &URLKitResolver{
		manager: opts.Manager,

		defaultGroup: strings.TrimSpace(opts.DefaultGroup),
		localeGroups: opts.LocaleGroups,

		defaultRoute:  strings.TrimSpace(opts.DefaultRoute),
		slugParam:     opts.SlugParam,
		localeParam:   strings.TrimSpace(opts.LocaleParam),
		localeIDParam: strings.TrimSpace(opts.LocaleIDParam),
		routeField:    strings.TrimSpace(opts.RouteField),
		paramsField:   strings.TrimSpace(opts.ParamsField),
		queryField:    strings.TrimSpace(opts.QueryField),

		groupCache: make(map[string]*urlkit.Group),
	}
}

// Resolve builds a URL using the configured route manager.
func (r *URLKitResolver) Resolve(ctx context.Context, req ResolveRequest) (string, error) {
	_ = ctx // reserved for future use
	if r == nil || r.manager == nil || req.Item == nil {
		return "", nil
	}

	groupPath := r.defaultGroup
	localeKey := strings.ToLower(strings.TrimSpace(req.Locale))
	if r.localeGroups != nil {
		if path, ok := r.localeGroups[localeKey]; ok && strings.TrimSpace(path) != "" {
			groupPath = strings.TrimSpace(path)
		}
	}
	if groupPath == "" {
		return "", nil
	}

	group, err := r.groupForPath(groupPath)
	if err != nil || group == nil {
		return "", err
	}

	routeName := r.defaultRoute
	if r.routeField != "" {
		if raw, ok := req.Item.Target[r.routeField]; ok {
			if str := strings.TrimSpace(fmt.Sprint(raw)); str != "" {
				routeName = str
			}
		}
	}
	if routeName == "" {
		return "", nil
	}

	builder, err := r.safeBuilder(group, routeName)
	if err != nil {
		return "", err
	}

	for key, val := range r.collectParams(req) {
		builder.WithParam(key, val)
	}

	for key, values := range r.collectQueries(req) {
		for _, v := range values {
			builder.WithQuery(key, v)
		}
	}

	url, err := builder.Build()
	if err != nil {
		return "", err
	}
	return url, nil
}

func (r *URLKitResolver) collectParams(req ResolveRequest) map[string]any {
	params := make(map[string]any)
	if r.slugParam != "" {
		if slug, ok := extractSlug(req.Item.Target); ok && slug != "" {
			params[r.slugParam] = slug
		}
	}

	if r.localeParam != "" && strings.TrimSpace(req.Locale) != "" {
		params[r.localeParam] = strings.TrimSpace(req.Locale)
	}

	if r.localeIDParam != "" && req.LocaleID != uuid.Nil {
		params[r.localeIDParam] = req.LocaleID.String()
	}

	if r.paramsField != "" {
		if raw, ok := req.Item.Target[r.paramsField]; ok {
			for key, val := range cloneAnyMap(raw) {
				params[key] = val
			}
		}
	}

	return params
}

func (r *URLKitResolver) collectQueries(req ResolveRequest) map[string][]string {
	queries := make(map[string][]string)
	if r.queryField == "" {
		return queries
	}
	raw, ok := req.Item.Target[r.queryField]
	if !ok {
		return queries
	}

	switch value := raw.(type) {
	case map[string]string:
		for k, v := range value {
			queries[k] = append(queries[k], v)
		}
	case map[string]any:
		for k, v := range value {
			switch tv := v.(type) {
			case []string:
				queries[k] = append(queries[k], tv...)
			case []any:
				for _, item := range tv {
					queries[k] = append(queries[k], fmt.Sprint(item))
				}
			default:
				queries[k] = append(queries[k], fmt.Sprint(tv))
			}
		}
	case map[string][]string:
		for k, v := range value {
			queries[k] = append(queries[k], v...)
		}
	}
	return queries
}

func (r *URLKitResolver) groupForPath(path string) (*urlkit.Group, error) {
	r.mu.RLock()
	group, ok := r.groupCache[path]
	r.mu.RUnlock()
	if ok {
		return group, nil
	}

	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("menus: invalid route group path %q", path)
	}

	root, err := lookupGroup(r.manager, parts[0])
	if err != nil {
		return nil, err
	}
	current := root
	for _, part := range parts[1:] {
		current, err = lookupChildGroup(current, part)
		if err != nil {
			return nil, err
		}
	}

	r.mu.Lock()
	r.groupCache[path] = current
	r.mu.Unlock()
	return current, nil
}

func (r *URLKitResolver) safeBuilder(group *urlkit.Group, route string) (*urlkit.Builder, error) {
	if group == nil {
		return nil, fmt.Errorf("menus: urlkit group is nil")
	}
	var (
		builder *urlkit.Builder
		err     error
	)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("menus: urlkit builder panic: %v", rec)
		}
	}()
	builder = group.Builder(route)
	return builder, err
}

func lookupGroup(manager *urlkit.RouteManager, name string) (*urlkit.Group, error) {
	if manager == nil {
		return nil, fmt.Errorf("menus: route manager not configured")
	}
	var (
		group *urlkit.Group
		err   error
	)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("menus: route group %q not found", name)
		}
	}()
	group = manager.Group(name)
	return group, err
}

func lookupChildGroup(parent *urlkit.Group, name string) (*urlkit.Group, error) {
	if parent == nil {
		return nil, fmt.Errorf("menus: parent group is nil")
	}
	var (
		group *urlkit.Group
		err   error
	)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("menus: child group %q not found", name)
		}
	}()
	group = parent.Group(name)
	return group, err
}

func cloneAnyMap(raw any) map[string]any {
	result := make(map[string]any)
	switch values := raw.(type) {
	case map[string]any:
		for k, v := range values {
			result[k] = v
		}
	case map[string]string:
		for k, v := range values {
			result[k] = v
		}
	}
	return result
}

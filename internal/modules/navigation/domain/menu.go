package domain

import (
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MaxMenuDepth = 8
	MaxMenuItems = 500
)

var (
	ErrInvalidMenu       = errors.New("navigation: invalid menu")
	ErrInvalidMenuItem   = errors.New("navigation: invalid menu item")
	ErrMenuTreeCycle     = errors.New("navigation: menu tree cycle")
	ErrMenuTreeTooDeep   = errors.New("navigation: menu tree is too deep")
	ErrMenuTreeParent    = errors.New("navigation: menu tree parent is missing")
	ErrMenuTreeDuplicate = errors.New("navigation: menu tree has duplicate ids")
	ErrMenuTreeOrder     = errors.New("navigation: menu tree sibling order is invalid")
)

type TargetKind string

const (
	TargetInternalPath TargetKind = "internal_path"
	TargetExternalURL  TargetKind = "external_url"
	TargetContentKey   TargetKind = "content_key"
)

type Menu struct {
	ID        string
	AccountID string
	LocaleID  string
	Key       string
	Name      string
	Enabled   bool
	Version   int64
	Items     []MenuItem
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MenuInput struct {
	ID        string
	AccountID string
	LocaleID  string
	Key       string
	Name      string
	Enabled   bool
	Version   int64
	Items     []MenuItem
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MenuItem struct {
	ID         string
	AccountID  string
	MenuID     string
	ParentID   string
	Label      string
	TargetKind TargetKind
	Target     string
	Position   int64
	Enabled    bool
	Version    int64
	Children   []MenuItem
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MenuItemInput struct {
	ID         string
	AccountID  string
	MenuID     string
	ParentID   string
	Label      string
	TargetKind TargetKind
	Target     string
	Position   int64
	Enabled    bool
	Version    int64
	Children   []MenuItem
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	identityPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	menuKeyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,127}$`)
	contentKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,127}$`)
	pathPattern       = regexp.MustCompile(`^/[A-Za-z0-9/_-]*$`)
)

func NewMenu(input MenuInput) (Menu, error) {
	key := strings.ToLower(strings.TrimSpace(input.Key))
	name := strings.TrimSpace(input.Name)
	if !validIdentity(input.ID) || !validIdentity(input.AccountID) || !validIdentity(input.LocaleID) ||
		!menuKeyPattern.MatchString(key) || name == "" || len(name) > 255 ||
		input.Version < 1 || (!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) {
		return Menu{}, ErrInvalidMenu
	}
	return Menu{
		ID: input.ID, AccountID: input.AccountID, LocaleID: input.LocaleID, Key: key, Name: name,
		Enabled: input.Enabled, Version: input.Version, Items: cloneItems(input.Items),
		DeletedAt: cloneTime(input.DeletedAt), CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC(),
	}, nil
}

func NewMenuItem(input MenuItemInput) (MenuItem, error) {
	id, accountID, menuID := strings.TrimSpace(input.ID), strings.TrimSpace(input.AccountID), strings.TrimSpace(input.MenuID)
	parentID := strings.TrimSpace(input.ParentID)
	label, target := strings.TrimSpace(input.Label), strings.TrimSpace(input.Target)
	if !validIdentity(id) || !validIdentity(accountID) || !validIdentity(menuID) ||
		(parentID != "" && !validIdentity(parentID)) || label == "" || len(label) > 255 ||
		input.Position < 0 || input.Version < 1 || len(target) == 0 || len(target) > 1024 ||
		(!input.CreatedAt.IsZero() && !input.UpdatedAt.IsZero() && input.UpdatedAt.Before(input.CreatedAt)) ||
		!validTarget(input.TargetKind, target) {
		return MenuItem{}, ErrInvalidMenuItem
	}
	return MenuItem{
		ID: id, AccountID: accountID, MenuID: menuID, ParentID: parentID, Label: label,
		TargetKind: input.TargetKind, Target: target, Position: input.Position, Enabled: input.Enabled,
		Version: input.Version, Children: cloneItems(input.Children), DeletedAt: cloneTime(input.DeletedAt),
		CreatedAt: input.CreatedAt.UTC(), UpdatedAt: input.UpdatedAt.UTC(),
	}, nil
}

func ValidateTree(items []MenuItem, maxDepth int) error {
	if maxDepth < 1 {
		return ErrMenuTreeTooDeep
	}
	flat := make([]MenuItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	var walk func([]MenuItem, string, int) error
	walk = func(current []MenuItem, inheritedParent string, depth int) error {
		if depth > maxDepth {
			return ErrMenuTreeTooDeep
		}
		for _, item := range current {
			if _, exists := seen[item.ID]; exists {
				return ErrMenuTreeDuplicate
			}
			seen[item.ID] = struct{}{}
			if inheritedParent != "" && item.ParentID != "" && item.ParentID != inheritedParent {
				return ErrMenuTreeParent
			}
			if item.ParentID == "" && inheritedParent != "" {
				item.ParentID = inheritedParent
			}
			flat = append(flat, item)
			if len(flat) > MaxMenuItems {
				return ErrMenuTreeTooDeep
			}
			if err := walk(item.Children, item.ID, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(items, "", 1); err != nil {
		return err
	}
	byID := make(map[string]MenuItem, len(flat))
	for _, item := range flat {
		byID[item.ID] = item
		if item.ParentID != "" {
			if _, ok := byID[item.ParentID]; !ok {
				// The parent may appear later in a flat representation; defer this check.
				continue
			}
		}
	}
	for _, item := range flat {
		if item.ParentID != "" {
			if _, ok := byID[item.ParentID]; !ok {
				return ErrMenuTreeParent
			}
		}
	}
	state := make(map[string]uint8, len(flat))
	depths := make(map[string]int, len(flat))
	var visit func(string) (int, error)
	visit = func(id string) (int, error) {
		switch state[id] {
		case 1:
			return 0, ErrMenuTreeCycle
		case 2:
			return depths[id], nil
		}
		state[id] = 1
		item := byID[id]
		depth := 1
		if item.ParentID != "" {
			parentDepth, err := visit(item.ParentID)
			if err != nil {
				return 0, err
			}
			depth = parentDepth + 1
		}
		if depth > maxDepth {
			return 0, ErrMenuTreeTooDeep
		}
		depths[id] = depth
		state[id] = 2
		return depth, nil
	}
	for _, item := range flat {
		if _, err := visit(item.ID); err != nil {
			return err
		}
	}
	if err := validateSiblingOrder(flat); err != nil {
		return err
	}
	return nil
}

func BuildTree(items []MenuItem) ([]MenuItem, error) {
	if err := ValidateTree(items, MaxMenuDepth); err != nil {
		return nil, err
	}
	flat := flattenItems(items)
	byID := make(map[string]MenuItem, len(flat))
	for _, item := range flat {
		item.Children = nil
		byID[item.ID] = item
	}
	roots := make([]MenuItem, 0)
	children := make(map[string][]MenuItem)
	for _, item := range byID {
		if item.ParentID == "" {
			roots = append(roots, item)
		} else {
			children[item.ParentID] = append(children[item.ParentID], item)
		}
	}
	var attach func(MenuItem) MenuItem
	attach = func(item MenuItem) MenuItem {
		list := children[item.ID]
		sort.SliceStable(list, func(i, j int) bool { return itemLess(list[i], list[j]) })
		item.Children = make([]MenuItem, 0, len(list))
		for _, child := range list {
			item.Children = append(item.Children, attach(child))
		}
		return item
	}
	sort.SliceStable(roots, func(i, j int) bool { return itemLess(roots[i], roots[j]) })
	for index := range roots {
		roots[index] = attach(roots[index])
	}
	return roots, nil
}

func FlattenItems(items []MenuItem) []MenuItem {
	return flattenItems(items)
}

func flattenItems(items []MenuItem) []MenuItem {
	flat := make([]MenuItem, 0, len(items))
	var nested func([]MenuItem, string)
	nested = func(current []MenuItem, inheritedParent string) {
		for _, item := range current {
			children := item.Children
			item.Children = nil
			if item.ParentID == "" && inheritedParent != "" {
				item.ParentID = inheritedParent
			}
			flat = append(flat, item)
			nested(children, item.ID)
		}
	}
	flat = flat[:0]
	nested(items, "")
	return flat
}

func cloneItems(items []MenuItem) []MenuItem {
	if len(items) == 0 {
		return nil
	}
	result := make([]MenuItem, len(items))
	for index, item := range items {
		result[index] = item
		result[index].Children = cloneItems(item.Children)
		result[index].DeletedAt = cloneTime(item.DeletedAt)
	}
	return result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func validIdentity(value string) bool { return identityPattern.MatchString(strings.TrimSpace(value)) }

func validTarget(kind TargetKind, target string) bool {
	switch kind {
	case TargetInternalPath:
		return pathPattern.MatchString(target) && !strings.Contains(target, "//")
	case TargetExternalURL:
		parsed, err := url.Parse(target)
		return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
	case TargetContentKey:
		return contentKeyPattern.MatchString(strings.ToLower(target)) && target == strings.ToLower(target)
	default:
		return false
	}
}

func validateSiblingOrder(items []MenuItem) error {
	positions := make(map[string]map[int64]struct{})
	for _, item := range items {
		if positions[item.ParentID] == nil {
			positions[item.ParentID] = make(map[int64]struct{})
		}
		if _, exists := positions[item.ParentID][item.Position]; exists {
			return ErrMenuTreeOrder
		}
		positions[item.ParentID][item.Position] = struct{}{}
	}
	return nil
}

func itemLess(left, right MenuItem) bool {
	if left.Position == right.Position {
		return left.ID < right.ID
	}
	return left.Position < right.Position
}

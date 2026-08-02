package domain

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3B4MenuValidatesTargetsAndBuildsOrderedTree(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	menu, err := NewMenu(MenuInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		LocaleID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", Key: "main", Name: "Main navigation",
		Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	root, err := NewMenuItem(MenuItemInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", AccountID: menu.AccountID, MenuID: menu.ID,
		Label: "Home", TargetKind: TargetInternalPath, Target: "/home", Position: 0,
		Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	child, err := NewMenuItem(MenuItemInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67584", AccountID: menu.AccountID, MenuID: menu.ID,
		ParentID: root.ID, Label: "Docs", TargetKind: TargetExternalURL, Target: "https://docs.example.test",
		Position: 0, Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	menu.Items = []MenuItem{root, child}
	require.NoError(t, ValidateTree(menu.Items, MaxMenuDepth))
	require.Equal(t, TargetInternalPath, menu.Items[0].TargetKind)
}

func TestM3B4TreeRejectsCycleDepthAndUnsafeTarget(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	first, err := NewMenuItem(MenuItemInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		MenuID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67582", Label: "One", TargetKind: TargetInternalPath,
		Target: "/one", Position: 0, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	second, err := NewMenuItem(MenuItemInput{
		ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67583", AccountID: first.AccountID, MenuID: first.MenuID,
		ParentID: first.ID, Label: "Two", TargetKind: TargetInternalPath, Target: "/two", Position: 0,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	second.ParentID = second.ID
	require.ErrorIs(t, ValidateTree([]MenuItem{first, second}, MaxMenuDepth), ErrMenuTreeCycle)

	unsafe := second
	unsafe.ParentID = ""
	unsafe.TargetKind = TargetExternalURL
	unsafe.Target = "javascript:alert(1)"
	_, err = NewMenuItem(MenuItemInput{
		ID: unsafe.ID, AccountID: unsafe.AccountID, MenuID: unsafe.MenuID, Label: unsafe.Label,
		TargetKind: unsafe.TargetKind, Target: unsafe.Target, Position: unsafe.Position,
		Version: unsafe.Version, CreatedAt: unsafe.CreatedAt, UpdatedAt: unsafe.UpdatedAt,
	})
	require.ErrorIs(t, err, ErrInvalidMenuItem)

	deep := make([]MenuItem, 0, MaxMenuDepth+1)
	parent := ""
	for index := 0; index <= MaxMenuDepth; index++ {
		item, itemErr := NewMenuItem(MenuItemInput{
			ID: menuTestID(index), AccountID: first.AccountID, MenuID: first.MenuID, ParentID: parent,
			Label: "Depth", TargetKind: TargetInternalPath, Target: "/depth", Position: 0,
			Version: 1, CreatedAt: now, UpdatedAt: now,
		})
		require.NoError(t, itemErr)
		deep = append(deep, item)
		parent = item.ID
	}
	require.ErrorIs(t, ValidateTree(deep, MaxMenuDepth), ErrMenuTreeTooDeep)
}

func menuTestID(index int) string {
	return fmt.Sprintf("018f1f64-7b2a-7c10-8bf2-000000000%03d", index)
}

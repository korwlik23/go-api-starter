package seed

import "github.com/google/uuid"

const seedNamespaceName = "go-lang-starter.local-seed.v1"

type IDs struct {
	Account   string
	OwnerUser string
}

func NewIDs() IDs {
	namespace := uuid.NewSHA1(uuid.NameSpaceURL, []byte(seedNamespaceName))
	return IDs{
		Account:   seedID(namespace, "account/personal"),
		OwnerUser: seedID(namespace, "user/owner"),
	}
}

func (ids IDs) User(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "user/"+key)
}

func (ids IDs) Membership(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "membership/"+key)
}

func (ids IDs) Role(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "role/"+key)
}

func (ids IDs) SystemRole(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "system-role/"+key)
}

func (ids IDs) Assignment(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "assignment/"+key)
}

func (ids IDs) Permission(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "permission/"+key)
}

func (ids IDs) Locale(tag string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "locale/"+tag)
}

func (ids IDs) Content(key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "content/"+key)
}

func (ids IDs) Translation(contentKey, localeTag string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "translation/"+contentKey+"/"+localeTag)
}

func (ids IDs) Revision(contentKey, localeTag string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "revision/"+contentKey+"/"+localeTag)
}

func (ids IDs) Taxonomy(kind, slug string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "taxonomy/"+kind+"/"+slug)
}

func (ids IDs) Menu(localeTag string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "menu/"+localeTag)
}

func (ids IDs) MenuItem(localeTag, key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, "menu-item/"+localeTag+"/"+key)
}

func (ids IDs) Record(kind, key string) string {
	namespace := uuid.MustParse(ids.Account)
	return seedID(namespace, kind+"/"+key)
}

func seedID(namespace uuid.UUID, name string) string {
	return uuid.NewSHA1(namespace, []byte(name)).String()
}

// Package shared holds the constants used by more than one console module.
// Console modules are independent Go packages and must not import each
// other; anything two or more modules need belongs here.
package shared

// PublishStatus marks the listing lifecycle of marketplace-style content
// (datasets, training courses, incubation programs).
const (
	PublishStatusDraft     = 0
	PublishStatusPublished = 1
	PublishStatusOffline   = 2
)

// ReviewStatus marks request-style flows that an administrator reviews
// (dataset grants, training enrollments, incubation applications).
const (
	ReviewStatusPending   = 0
	ReviewStatusReviewing = 1
	ReviewStatusApproved  = 2
	ReviewStatusRejected  = 3
)

// PriceMode distinguishes free listings from paid ones. Billing settlement
// for paid items is a follow-up; the field records the intent only.
const (
	PriceModeFree = 0
	PriceModePaid = 1
)

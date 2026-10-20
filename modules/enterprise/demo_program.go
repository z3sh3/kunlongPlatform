package enterprise

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"

	"github.com/gin-gonic/gin"
)

// Bundled demonstration programs for the incubation console.
//
// Three published programs cover all program types (enterprise incubation, SME
// support, government/park programs). Each one is paired with an application in
// a different review state, so an administrator can walk the whole flow —
// pending → reviewing → approved — on a fresh deployment without preparing any
// data by hand. The cover images ship with the frontend under /covers.

type demoProgram struct {
	Title        string
	Type         string
	Summary      string
	CoverUrl     string
	Description  string
	Requirements string

	// Application fields seed one application for the acting administrator,
	// one per review stage.
	CompanyName      string
	ContactName      string
	ContactPhone     string
	ApplicationNote  string
	ApplicationState int
	ReviewComment    string
}

var demoPrograms = []demoProgram{
	{
		Title:        "AI 创业孵化营",
		Type:         TypeIncubation,
		Summary:      "12 周孵化周期，提供算力额度、导师辅导与融资对接。",
		CoverUrl:     "/covers/enterprise-incubation-camp.svg",
		Description:  "面向早期 AI 创业团队，提供算力额度、产品打磨、商业模式梳理与投资人对接，结营时进行路演评审。",
		Requirements: "已注册公司，核心团队 2 人以上，具备可演示的 AI 产品原型。",

		CompanyName:      "星澜智能科技",
		ContactName:      "王琳",
		ContactPhone:     "13800000001",
		ApplicationNote:  "面向中小企业的 AI 客服解决方案，已有 3 家种子客户，希望入营打磨产品与商业模式。",
		ApplicationState: shared.ReviewStatusPending,
	},
	{
		Title:        "中小企业 AI 降本增效专项",
		Type:         TypeSMESupport,
		Summary:      "面向中小企业的 AI 工具补贴与落地辅导，单个项目最高补贴 50%。",
		CoverUrl:     "/covers/enterprise-sme-support.svg",
		Description:  "为中小企业提供 AI 工具选型、流程改造辅导与费用补贴，按项目验收结果分阶段拨付。",
		Requirements: "年营收 5000 万元以下，有明确的业务流程改造需求与专职对接人。",

		CompanyName:      "云枢数据服务",
		ContactName:      "李哲",
		ContactPhone:     "13800000002",
		ApplicationNote:  "为制造业客户提供设备预测性维护，希望通过专项降低交付成本并拓展渠道。",
		ApplicationState: shared.ReviewStatusReviewing,
	},
	{
		Title:        "高新区 AI 产业培育计划",
		Type:         TypeGovPark,
		Summary:      "园区专项：算力券、场景开放与产业链上下游对接。",
		CoverUrl:     "/covers/enterprise-gov-park.svg",
		Description:  "面向高新区内 AI 企业的培育计划，提供算力券、公共场景开放、产业链对接与人才公寓等配套支持。",
		Requirements: "注册地在高新区，主营业务属于人工智能相关领域，近一年无重大失信记录。",

		CompanyName:      "智谷机器人",
		ContactName:      "陈晨",
		ContactPhone:     "13800000003",
		ApplicationNote:  "园区内协作机器人企业，希望获得算力补贴与产业对接支持。",
		ApplicationState: shared.ReviewStatusApproved,
		ReviewComment:    "已通过初审，等待入营通知。",
	},
}

type demoProgramSeedResult struct {
	Created      []IncubationProgram `json:"created"`
	Existing     []string            `json:"existing"`
	Applications int                 `json:"applications"`
}

// SeedDemoPrograms publishes the bundled demo programs and one application per
// program for the acting administrator. It is idempotent: a program whose title
// already exists is reported, and an existing application is never re-created
// or modified.
func SeedDemoPrograms(c *gin.Context) {
	userId := c.GetInt("id")
	titles := make([]string, 0, len(demoPrograms))
	for _, demo := range demoPrograms {
		titles = append(titles, demo.Title)
	}

	var found []IncubationProgram
	if err := model.DB.Where("title IN ?", titles).Find(&found).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	byTitle := make(map[string]IncubationProgram, len(found))
	for _, program := range found {
		byTitle[program.Title] = program
	}

	result := demoProgramSeedResult{Created: []IncubationProgram{}, Existing: []string{}}
	for _, demo := range demoPrograms {
		if _, ok := byTitle[demo.Title]; ok {
			result.Existing = append(result.Existing, demo.Title)
			continue
		}
		program := IncubationProgram{
			Title:        demo.Title,
			Type:         demo.Type,
			Summary:      shared.NormalizeSummary(demo.Summary),
			CoverUrl:     shared.NormalizeCoverUrl(demo.CoverUrl),
			Description:  demo.Description,
			Requirements: demo.Requirements,
			Status:       shared.PublishStatusPublished,
		}
		if err := model.DB.Create(&program).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		byTitle[program.Title] = program
		result.Created = append(result.Created, program)
	}

	// Programs published by an earlier run still need their demo application,
	// so the application step covers every bundled program, not only new ones.
	programIds := make([]int, 0, len(demoPrograms))
	for _, demo := range demoPrograms {
		programIds = append(programIds, byTitle[demo.Title].Id)
	}
	var applied []IncubationApplication
	if err := model.DB.Where("user_id = ? AND program_id IN ?", userId, programIds).
		Find(&applied).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	hasApplication := make(map[int]bool, len(applied))
	for _, application := range applied {
		hasApplication[application.ProgramId] = true
	}

	pending := make([]IncubationApplication, 0, len(demoPrograms))
	for _, demo := range demoPrograms {
		programId := byTitle[demo.Title].Id
		if hasApplication[programId] {
			continue
		}
		pending = append(pending, IncubationApplication{
			ProgramId:     programId,
			UserId:        userId,
			CompanyName:   demo.CompanyName,
			ContactName:   demo.ContactName,
			ContactPhone:  demo.ContactPhone,
			Description:   demo.ApplicationNote,
			Status:        demo.ApplicationState,
			ReviewComment: demo.ReviewComment,
		})
	}
	if len(pending) > 0 {
		if err := model.DB.Create(&pending).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	}
	result.Applications = len(pending)
	common.ApiSuccess(c, result)
}

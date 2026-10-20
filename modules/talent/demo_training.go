package talent

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modules/shared"

	"github.com/gin-gonic/gin"
)

// Bundled demonstration courses for the training console.
//
// Three published courses cover both categories (certification training and
// training consultation). Each one is paired with an enrollment in a different
// review state, so an administrator can walk the whole flow — pending →
// approved → completed — on a fresh deployment without preparing any data by
// hand. The cover images ship with the frontend under /covers.

type demoCourse struct {
	Title       string
	Category    string
	Summary     string
	CoverUrl    string
	Description string
	Content     string
	Schedule    string
	Capacity    int
	PriceMode   int
	Price       int64
	// EnrollmentStatus and EnrollmentRemark seed one enrollment for the acting
	// administrator, one per review stage.
	EnrollmentStatus int
	EnrollmentRemark string
}

var demoCourses = []demoCourse{
	{
		Title:       "大模型应用工程师认证（初级）",
		Category:    CategoryCertification,
		Summary:     "3 天集训 + 认证考试，覆盖提示词工程、RAG 与智能体开发。",
		CoverUrl:    "/covers/talent-llm-certification.svg",
		Description: "面向具备基础编程能力的工程师，帮助学员独立完成大模型应用的方案设计、检索增强与上线评测，通过考试后获得平台认证证书。",
		Content:     "第一天：提示词工程与效果评测；第二天：RAG 检索增强与知识库建设；第三天：智能体与工具调用；第四天：认证笔试与实操答辩。",
		Schedule:    "每月 5 日开班，线上 3 天 + 线下 1 天。",
		Capacity:    40,
		PriceMode:   shared.PriceModeFree,
		Price:       0,
		// Pending: the administrator can approve it and then mark it completed.
		EnrollmentStatus: shared.ReviewStatusPending,
	},
	{
		Title:       "生成式 AI 产品经理实战研修",
		Category:    CategoryCertification,
		Summary:     "从需求洞察到商业化落地，4 个半天打通 AI 产品全链路。",
		CoverUrl:    "/covers/talent-ai-product-manager.svg",
		Description: "面向产品经理与业务负责人，用真实案例练习 AI 需求评估、方案选型、成本测算与灰度上线，结课输出一份可评审的产品方案。",
		Content:     "模块一：AI 能力边界与需求筛选；模块二：方案选型与成本测算；模块三：数据与合规风险；模块四：灰度上线与效果度量。",
		Schedule:    "双周末线上直播，共 4 个半天。",
		Capacity:    30,
		PriceMode:   shared.PriceModePaid,
		Price:       5000000,
		// Approved: the next step in the flow is marking it completed.
		EnrollmentStatus: shared.ReviewStatusApproved,
		EnrollmentRemark: "资料已发放，等待开班。",
	},
	{
		Title:       "企业 AI 转型咨询工作坊",
		Category:    CategoryConsultation,
		Summary:     "面向管理层的 2 天工作坊，输出可执行的 AI 转型路线图。",
		CoverUrl:    "/covers/talent-consulting-workshop.svg",
		Description: "以企业真实业务为输入，通过 2 天集中工作坊梳理场景优先级、组织分工与投入预算，形成 90 天可落地的转型路线图。",
		Content:     "第一天：业务现状盘点与场景识别；第二天：优先级排序、投入测算与 90 天路线图评审。",
		Schedule:    "按企业预约排期，2 天线下工作坊。",
		Capacity:    20,
		PriceMode:   shared.PriceModeFree,
		Price:       0,
		// Completed: shows the terminal state of the enrollment flow.
		EnrollmentStatus: EnrollmentStatusCompleted,
		EnrollmentRemark: "已完成结业答辩。",
	},
}

type demoCourseSeedResult struct {
	Created     []TrainingCourse `json:"created"`
	Existing    []string         `json:"existing"`
	Enrollments int              `json:"enrollments"`
}

// SeedDemoCourses publishes the bundled demo courses and one enrollment per
// course for the acting administrator. It is idempotent: a course whose title
// already exists is reported, and an existing enrollment is never re-created
// or modified.
func SeedDemoCourses(c *gin.Context) {
	userId := c.GetInt("id")
	titles := make([]string, 0, len(demoCourses))
	for _, demo := range demoCourses {
		titles = append(titles, demo.Title)
	}

	var found []TrainingCourse
	if err := model.DB.Where("title IN ?", titles).Find(&found).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	byTitle := make(map[string]TrainingCourse, len(found))
	for _, course := range found {
		byTitle[course.Title] = course
	}

	result := demoCourseSeedResult{Created: []TrainingCourse{}, Existing: []string{}}
	for _, demo := range demoCourses {
		if _, ok := byTitle[demo.Title]; ok {
			result.Existing = append(result.Existing, demo.Title)
			continue
		}
		course := TrainingCourse{
			Title:       demo.Title,
			Category:    demo.Category,
			Summary:     shared.NormalizeSummary(demo.Summary),
			CoverUrl:    shared.NormalizeCoverUrl(demo.CoverUrl),
			Description: demo.Description,
			Content:     demo.Content,
			Schedule:    demo.Schedule,
			Capacity:    demo.Capacity,
			PriceMode:   demo.PriceMode,
			Price:       demo.Price,
			Status:      shared.PublishStatusPublished,
		}
		if err := model.DB.Create(&course).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		byTitle[course.Title] = course
		result.Created = append(result.Created, course)
	}

	// Courses published by an earlier run still need their demo enrollment, so
	// the enrollment step covers every bundled course, not only new ones.
	courseIds := make([]int, 0, len(demoCourses))
	for _, demo := range demoCourses {
		courseIds = append(courseIds, byTitle[demo.Title].Id)
	}
	var enrolled []TrainingEnrollment
	if err := model.DB.Where("user_id = ? AND course_id IN ?", userId, courseIds).
		Find(&enrolled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	hasEnrollment := make(map[int]bool, len(enrolled))
	for _, enrollment := range enrolled {
		hasEnrollment[enrollment.CourseId] = true
	}

	pending := make([]TrainingEnrollment, 0, len(demoCourses))
	for _, demo := range demoCourses {
		courseId := byTitle[demo.Title].Id
		if hasEnrollment[courseId] {
			continue
		}
		pending = append(pending, TrainingEnrollment{
			CourseId: courseId,
			UserId:   userId,
			Status:   demo.EnrollmentStatus,
			Remark:   demo.EnrollmentRemark,
		})
	}
	if len(pending) > 0 {
		if err := model.DB.Create(&pending).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	}
	result.Enrollments = len(pending)
	common.ApiSuccess(c, result)
}

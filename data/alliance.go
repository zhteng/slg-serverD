package data

type Alliance struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Icon        int    `json:"icon"`
	Level       int    `json:"level"`
	Notice      string `json:"notice"`
	Leader      int64  `json:"leader"`
	MemberCount int    `json:"member_count"`
	MaxMember   int    `json:"max_member"`
	JoinType    int    `json:"join_type"`
	CreateTime  int64  `json:"create_time"`
	IsDisband   int    `json:"is_disband"`
}

// 成员职位
const (
	MemberPosNormal = 0 // 普通
	MemberPosLeader = 1 // 盟主
	MemberPosVice   = 2 // 副盟主
)

// 入会类型常量
const (
	JoinTypeFree   = 1 // 自由加入
	JoinTypeApply  = 2 // 需申请
	JoinTypeRefuse = 3 // 拒绝所有人
)

type AllianceMember struct {
	Aid          int64 `json:"aid"`
	Uid          int64 `json:"uid"`
	Position     int   `json:"position"`
	Contribution int   `json:"contribution"`
	JoinTime     int64 `json:"join_time"`
	LastOnline   int64 `json:"last_online"`
	QuitCdTime   int64 `json:"quit_cd_time"`
}

// 申请
type AllianceApply struct {
	Id        int64 `json:"id"`
	Aid       int64 `json:"aid"`
	Uid       int64 `json:"uid"`
	ApplyTime int64 `json:"apply_time"`
}

const (
	LogCreate  = 1
	LogJoin    = 2
	LogQuit    = 3
	LogKick    = 4
	LogPos     = 5
	LogDisband = 6
)

type AllianceLog struct {
	Id        int64  `json:"id"`
	Aid       int64  `json:"aid"`
	Uid       int64  `json:"uid"`
	TargetUid int64  `json:"target_uid"`
	Type      int    `json:"type"`
	Content   string `json:"content"`
	LogTime   int64  `json:"log_time"`
}

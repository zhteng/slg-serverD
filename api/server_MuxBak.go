package api

/*
type Server struct {
	userSvc     *game.UserService
	allianceSvc *game.AllianceService
	buildingSvc *game.BuildingService
	troopsSvc   *game.TroopsService
	marchSvc    *game.MarchService
	hub         *ws.Hub
}

func NewServer(userSvc *game.UserService, allianceSvc *game.AllianceService, buildingSvc *game.BuildingService, troopsSvc *game.TroopsService, marchSvc *game.MarchService, hub *ws.Hub) *Server {
	return &Server{userSvc: userSvc, allianceSvc: allianceSvc, buildingSvc: buildingSvc, troopsSvc: troopsSvc, marchSvc: marchSvc, hub: hub}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/user/info", s.userInfo)
	mux.HandleFunc("/alliance/create", s.createAlliance)     // 创建军团
	mux.HandleFunc("/alliance/join", s.joinAlliance)         // Join 申请加入军团
	mux.HandleFunc("/alliance/approved", s.approvedAlliance) // 审批
	mux.HandleFunc("/alliance/kick", s.kickMember)           // 审批

	mux.HandleFunc("/building/upgrade", s.upgradeBuilding)

	mux.HandleFunc("/troops/train", s.startTrain) //

	mux.HandleFunc("/march/launch", s.launchMarch)
	mux.HandleFunc("/march/cancel_gather", s.cancelGather)

	mux.HandleFunc("/ws", ws.HandleWebSocket(s.hub))
}

func (s *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	uid, err := strconv.ParseInt(r.URL.Query().Get("uid"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid uid")
		return
	}

	u, err := s.userSvc.Load(r.Context(), uid)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	success(w, u)
}

func (s *Server) createAlliance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Icon int    `json:"icon"`
		Uid  int64  `json:"uid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	a, err := s.allianceSvc.Create(r.Context(), req.Name, req.Icon, req.Uid)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error()) // 业务错误也用400，符合原有逻辑
		return
	}
	success(w, a)
}

func (s *Server) joinAlliance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Aid int64 `json:"aid"`
		Uid int64 `json:"uid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.allianceSvc.Join(r.Context(), req.Aid, req.Uid)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	success(w, "success")
}

func (s *Server) approvedAlliance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Aid     int64 `json:"aid"`
		Uid     int64 `json:"uid"`
		ApplyId int64 `json:"apply_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.allianceSvc.Approved(r.Context(), req.Aid, req.Uid, req.ApplyId, 1)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	success(w, "success")
}

func (s *Server) kickMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperatorUid int64 `json:"operator_uid"`
		TargetUid   int64 `json:"target_uid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.allianceSvc.KickMember(r.Context(), req.OperatorUid, req.TargetUid); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	success(w, "ok")
}

func (s *Server) upgradeBuilding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Uid        int64  `json:"uid"`
		BuildingId string `json:"building_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.buildingSvc.Upgrade(r.Context(), req.Uid, req.BuildingId)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	success(w, "upgrade started")
}

func (s *Server) startTrain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Uid        int64  `json:"uid"`
		SoldierKey string `json:"soldier_key"`
		Num        int    `json:"num"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := s.troopsSvc.UpdateTroops(r.Context(), req.Uid, req.SoldierKey, req.Num)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	success(w, "train started")
}

// ------------------- 行军 -------------------
func (s *Server) launchMarch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Uid            int64          `json:"uid"`
		ToX            int            `json:"to_x"`
		ToY            int            `json:"to_y"`
		Type           int            `json:"type"`
		Troops         map[string]int `json:"troops"`
		GatherDuration int64          `json:"gather_duration,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}

	march, err := s.marchSvc.LaunchMarch(r.Context(), req.Uid, req.ToX, req.ToY, req.Type, req.Troops, req.GatherDuration)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	success(w, march)
}

func (s *Server) cancelGather(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Uid      int64  `json:"uid"`
		MarchKey string `json:"march_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.marchSvc.CancelGather(r.Context(), req.Uid, req.MarchKey); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	success(w, "gather canceled")
}
*/

# REQUIREMENTS — Milestone v1.2: Channel Model Management & Group UX

## Channel Model Management

- [ ] **CHN-01**: 管理员可在 Channel 列表中看到每个 channel 已配置的模型列表（内联展示）
- [ ] **CHN-02**: 管理员可在编辑 channel 时手动增删模型（操作 CustomModels 字段）
- [ ] **CHN-03**: 管理员可点击 channel 的 "Sync" 按钮触发从上游拉取 /v1/models
- [ ] **CHN-04**: Sync 弹窗展示上游模型列表，已在 CustomModels 中的模型预勾选
- [ ] **CHN-05**: 管理员选择要保留的模型后点击 Save，结果保存到 CustomModels（不覆盖未选中的已有记录之外——即结果 = 本次选中集合）
- [ ] **CHN-06**: 管理员可点击 channel 行的 "Duplicate" 按钮，创建同配置的新 channel（name 附加 " (copy)"，base_urls 和 keys 全部复制）

## Group UX

- [ ] **GRP-01**: 管理员在 Group 的添加/编辑 item 中，选完 channel 后 model_name 字段变为下拉框，选项来自该 channel 的已配模型（CustomModels ∪ Models）
- [ ] **GRP-02**: 若所选 channel 无模型列表，model_name 字段降级为文本输入（兼容旧数据）
- [ ] **GRP-03**: 管理员可点击 group 行的 "Duplicate" 按钮，创建同配置的新 group（name 附加 " (copy)"，items 全部复制）

## Out of Scope

- AutoSync 定时自动触发（手动触发已满足需求）
- 模型级别的熔断或健康检查
- 从外部 YAML/JSON 批量导入 channel

## Traceability

| REQ-ID | Phase | Status |
|--------|-------|--------|
| CHN-01 | Phase 6 | Pending |
| CHN-02 | Phase 6 | Pending |
| CHN-03 | Phase 7 | Pending |
| CHN-04 | Phase 7 | Pending |
| CHN-05 | Phase 7 | Pending |
| CHN-06 | Phase 9 | Pending |
| GRP-01 | Phase 8 | Pending |
| GRP-02 | Phase 8 | Pending |
| GRP-03 | Phase 9 | Pending |

# 跨文档链接完整性检查（2026-03-27）

| 文档 | 检查结果 | 备注 |
|---|---|---|
| docs/supply_button_level_prd_v1_2026-03-25.md | PASS | refs=6, missing=0 |
| docs/supply_test_plan_enhanced_v1_2026-03-25.md | PASS | refs=14, missing=0 |
| docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md | FAIL | refs=84, missing=31; docs/compat/canonical_endpoint_matrix.md; tests/compat/schema_gate_report.md; tests/compat/behavior_gate_report.md; scripts/gate/perf_gate_check.sh; docs/compat/risk_severity_playbook.md; sql/takeover_main_path_canonical.sql; docs/compat/cn_platform_mapping.md; scripts/security/config_hardening_scan.sh; tests/security/query_key_boundary_report.md; tests/security/credential_exposure_scan_report.md; docs/security/direct_supplier_call_detection_v1.md; reports/security/platform_credential_ingress_coverage_2026-03-26.md; docs/ops/unified_change_flow.md; scripts/release/rollback_subapi.sh; docs/runbook/subapi_integration_runbook_v1.md; reports/sprint_risk_control_review_2026-03-31.md; docs/product/migration_incident_comms_v1.md; docs/product/billing_dispute_sla_v1.md; reports/raci_snapshot_2026-03-18.md; reports/user_representative_migration_walkthrough_2026-03-25.md; reports/user_billing_dispute_drill_2026-03-25.md; tests/compat/contract_drift_ci_report.md; tests/compat/stream_failover_stress_report.md; evidence/*/wave_gate_bundle.md; tests/security/credential_boundary_regression_report.md; docs/gateway/provider_capability_matrix_v1.md; docs/gateway/degrade_playbook_v1.md; docs/gateway/adapter_spi_versioning_v1.md; platform_core_schema_v1.sql; reports/design_drift_daily_*.md; docs/token_runtime_minimal_spec_v1.md |
| review/prd_tech_planning_recheck_v3_2026-03-27.md | FAIL | refs=23, missing=2; platform_core_schema_v1.sql; supply_schema_v1_patch_2026-03-27.sql |
| review/superpowers_comprehensive_planning_review_v1_2026-03-25.md | PASS | refs=10, missing=0 |
| reports/superpowers_execution_progress_2026-03-27.md | PASS | refs=6, missing=0 |
| reports/alignment_validation_checkpoint_01_2026-03-27.md | PASS | refs=8, missing=0 |
| reports/alignment_validation_checkpoint_02_2026-03-27.md | PASS | refs=3, missing=0 |
| reports/alignment_validation_checkpoint_03_2026-03-27.md | PASS | refs=1, missing=0 |
| reports/alignment_validation_checkpoint_04_2026-03-27.md | FAIL | refs=4, missing=1; staging_precheck_and_run.sh |
| reports/alignment_validation_checkpoint_05_2026-03-27.md | PASS | refs=3, missing=0 |

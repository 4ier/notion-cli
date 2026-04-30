# CHANGELOG

모든 Git 커밋 이력을 최신순으로 기록합니다. 새 커밋은 표 최상단에 추가합니다.

| 일시 | 유형 | 범위 | 변경내용 (목적 포함) |
|---|---|---|---|
| 2026-04-30 13:00 | feat | auth,client | `auth status`/`doctor`에 integration type 노출 + workspace-root 생성 에러에 concrete workaround 힌트 추가 (#25) |
| 2026-04-30 13:00 | feat | file | `notion file upload`이 stdin(`-`)과 http(s) URL을 소스로 받도록 확장, `--name` 오버라이드 플래그 추가 (#26) |
| 2026-04-30 13:00 | feat | block | `--image-file`/`--image-upload` 및 file/video/audio/pdf 5종 미디어 플래그 패밀리 추가 — 업로드-임베드 워크플로우 원-커맨드화 (#23) |
| 2026-04-30 13:00 | feat | block | children.length>100 자동 배칭 + rich_text 2000자 한계 초과 시 코드블록 줄바꿈 기준 자동 분할, `--on-oversize=split\|truncate\|fail` 플래그 추가 (#21) |
| 2026-04-30 13:00 | feat | block | 마크다운 코드펜스 언어 별칭(ts/sh/yml/py/…) Notion enum으로 자동 정규화, 미등록 언어는 plain text로 경고와 함께 폴백 (#22) |
| 2026-04-30 13:00 | fix | api | `/v1` 접두사 자동 보정, `--body @file`·`--body -` 지원, help 예제 실제 동작과 일치하도록 정리 (#24) |
| 2026-03-14 21:00 | chore | gitignore | notion.exe 바이너리 gitignore 추가 — Windows 빌드 결과물 추적 방지 |
| 2026-03-14 20:10 | fix | block | table_row children을 table{} 내부로 이동 — Notion API 스펙 준수 ('table.children should be defined' 오류 수정) |
| 2026-03-14 19:52 | feat | block | GFM 테이블 파싱 + 인라인 서식(bold/italic/code/link/strike) 지원 추가 — 노션 CLI로 마크다운 표 업로드 시 깨지던 문제 근본 해결 |

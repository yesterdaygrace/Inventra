# Wave 4 Verification — Administration: Users + Activity Logs + Settings

## Scope
Per `.omo/plans/frontend-waves.md` Wave 4: users table (list/role/deactivate),
activity log viewer, settings + profile (change password, update profile).
Built against the live `:8090` demo backend.

## Frontend changes
- `web/src/lib/api.ts` — added `activityApi.list()` (GET `/activity-logs`, admin
  endpoint, paginated with filters); `Activity` type already existed.
- `web/src/pages/users.tsx` — admin users list (name/email, role badge,
  status badge, actions dropdown: change role dialog + deactivate with
  confirmation); debounced name search, role/status filter, pagination;
  prevents action on the logged-in user's own row.
- `web/src/pages/activity.tsx` — paginated audit-trail log (time, user,
  action, entity type, truncated details, IP); action text + entity-type
  filters, pagination.
- `web/src/pages/settings.tsx` — Profile card (name/email, RHF+zod,
  success banner, `setUser` updates sidebar live); Change password card
  (old/new/confirm, RHF+zod with password-match refinement, backend 401
  surfaced inline on wrong current password).
- `web/src/App.tsx` — `/users` (admin-gated), `/activity` (admin-gated),
  `/settings` (any authenticated) now render real pages; removed dead
  `ConstructingPage` import and deleted `placeholder.tsx`.

## Build
- `npx tsc -b --noEmit` clean; `npm run build` green (2389 modules).

## Live verification (Playwright, admin)
### Users
- 2 users rendered (Demo User STAFF + System Administrator ADMIN); role +
  status filters present; actions menu opens for the demo user's row.
- **Role change**: changed Demo User from STAFF → ADMIN via the Change role
  dialog → toast "Role updated" shown; table reflects the new role.
- Reverted demo user back to STAFF via `PUT /users/:id/role` after the test
  to restore seed state.
### Activity Log
- 10 seeded audit events rendered; action + entity-type filters present.
- Stock-in and role-change events recorded.
### Settings
- Profile form prefilled with the logged-in user's name + email.
- **Wrong old password**: change-password with an incorrect current password
  → backend `unauthorized` surfaced in the dialog alert (no duplicate toast).
- **Client mismatch**: mismatched confirm/new → zod refinement shows
  "Passwords do not match" inline.
- **Successful profile edit**: changed name to "QA Tester" → toast
  "Profile updated"; sidebar immediately reflected "QA Tester";
  reverted to "System Administrator" and confirmed saved.

## Evidence
- `demo-17-users.png`, `demo-18-activity.png`, `demo-19-settings.png`.

## Notes
- Backend unchanged this wave (no API bugs surfaced; user/activity/settings
  endpoints were already contract-correct from wave 1).
- The stored browser session dropped mid-test (15-minute access-token TTL);
  re-login succeeded cleanly — no functional issue.

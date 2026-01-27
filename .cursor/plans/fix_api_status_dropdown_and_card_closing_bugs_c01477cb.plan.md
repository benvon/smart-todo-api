---
name: Fix API status dropdown and card closing bugs
overview: Add clickable API status dropdown with real-time extended health info, fix profile context save error, and prevent cards from closing on focus changes by using explicit save/close buttons.
todos:
  - id: api-status-dropdown
    content: Add clickable API status dropdown with extended health info display
    status: completed
  - id: fix-profile-context-load
    content: Fix profile context loading to handle empty/null values correctly
    status: completed
  - id: fix-profile-context-save
    content: Fix profile context save error handling and payload format
    status: completed
  - id: remove-blur-handlers
    content: Remove blur event handlers from todo cards to prevent aggressive closing
    status: completed
  - id: test-card-interactions
    content: Test that cards only close via explicit buttons/actions
    status: completed
isProject: false
---

# Fix API Status Dropdown and Card Closing Bugs

## Overview

This plan addresses three issues:

1. Make API status clickable to show extended health information in real-time
2. Fix profile context save error ("failed to fetch")
3. Prevent cards from closing on focus changes by using explicit save/close buttons

## Changes Required

### 1. API Status Dropdown Feature

**Files to modify:**

- `web/js/app.js` - Add click handler and dropdown functionality
- `web/js/api.js` - Add function to fetch extended health status
- `web/app.html` - Add dropdown container for extended status
- `web/css/style.css` - Style the dropdown

**Implementation:**

- Add click handler to `#api-status` element that toggles dropdown visibility
- Create `checkExtendedAPIHealth()` function in `api.js` that calls `/healthz?mode=extended`
- Display extended health info (status, timestamp, checks) in a dropdown panel
- Auto-refresh extended status every 2-3 seconds when dropdown is open
- Close dropdown when clicking outside

### 2. Fix Profile Context Save Error

**Files to modify:**

- `web/js/profile.js` - Fix context loading and saving logic
- `web/js/api.js` - Verify `updateAIContext` handles empty strings correctly

**Issues to investigate:**

- Check if `getAIContext()` response format matches expected structure
- Ensure `updateAIContext()` sends correct payload format
- Handle empty context_summary properly (empty string vs null)
- Improve error handling to show specific error messages instead of generic "failed to fetch"

**Implementation:**

- Verify context loading sets textarea value correctly even when context_summary is empty/null
- Ensure updateAIContext sends `{context_summary: ""}` for empty strings (not null/undefined)
- Add better error logging and user-facing error messages

### 3. Fix Aggressive Card Closing

**Files to modify:**

- `web/js/app.js` - Remove blur handlers, rely on explicit buttons
- `web/js/profile.js` - Ensure profile panel only closes on explicit actions

**For Todo Edit Cards:**

- Remove `blur` event listener from due date input (line 742)
- Keep Save/Cancel buttons as the only way to exit edit mode
- Prevent accidental closes when clicking buttons/fields inside the card

**For Profile Panel:**

- Ensure backdrop click still works (already implemented)
- Verify close button works correctly
- Make sure clicking inside the panel content doesn't close it

**Implementation:**

- Remove `input.addEventListener('blur', finishEdit)` from `handleEditDueDate()`
- Keep Enter/Escape key handlers for due date editing
- Ensure all card interactions use explicit buttons for closing
- Test that clicking buttons inside cards doesn't trigger unwanted closes

## Testing Checklist

- [ ] API status dropdown opens/closes correctly
- [ ] Extended health info displays correctly and updates in real-time
- [ ] Profile context loads correctly (even when empty)
- [ ] Profile context saves successfully without "failed to fetch" error
- [ ] Todo edit cards don't close on focus changes
- [ ] Profile panel doesn't close when clicking inside content
- [ ] Save/Cancel buttons work correctly for todo editing
- [ ] Due date editing still works with Enter/Escape keys
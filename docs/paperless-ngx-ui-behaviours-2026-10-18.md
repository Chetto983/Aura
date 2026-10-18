# Paperless-ngx UI: behaviours for Aura's document view (clean-room brief), 2026-10-18

Source: a shallow, sparse clone of `src-ui/src` and `docs` from `paperless-ngx` `dev` at
`961f7fd672eb` (2026-10-18), read only and never run.
- Paths are relative to `src-ui/src/app/` unless they start with `docs/`.
- Paperless is GPL-3.0 and Aura is MIT, so this file describes behaviour only. It copies no code,
  templates, styles, regexes or strings beyond short UI labels.
- The backend side is in `docs/paperless-ngx-behaviours-2026-10-17.md`.

**Two places where the UI departs from the backend:**
- New tags, correspondents and types default to the **Auto** algorithm in the UI
  (`data/matching-model.ts:10`), while the backend model defaults to Any.
- The custom-field query builder caps a query at depth 4 and 5 conditions per group
  (`data/custom-field-query.ts:136-137`), while the backend allows 10 and 20.

## a. Document list

- **Three views:** a table, small cards and large cards (`components/document-list/document-list.component.html:48-61`).
  - Small cards are the default. They overlay at most 7 tags and show "+N" for the rest
    (`document-card-small.component.ts:95-101`).
  - Large cards show an excerpt or the search highlights, a "More like this" button and a
    relevance bar (`document-card-large/*.html:48-61, 151-155`).
- **A "Show" menu toggles the columns per user.** The columns are title, created, added, tags,
  correspondent, type, storage path, notes, owner, shared, ASN, pages, and every custom field
  (`document-list.component.html:33-47`; `data/document.ts:11-76`). Tags render inside the title
  cell.
- **Sorting.**
  - Table headers sort, plus a Sort menu. Every custom field can sort
    (`services/rest/document.service.ts:123-136`).
  - In a full-text search the only order offered is the score (`data/document.ts:100-105`).
- **Pagination.** 50 per page (`data/ui-settings.ts:161-163`), and an "N documents (filtered)"
  counter beside "Reset filters" (`.html:132-146`).
- **A value is a filter link.** Clicking a tag, type or correspondent in a row toggles that filter
  (`document-list.component.ts:555-578`).
- **Selection.**
  - Click toggles a document, shift-click selects a range, and there are buttons for none, page
    and all.
  - "All" is every match across pages: a flag plus the excluded documents
    (`services/document-list-view.service.ts:708-746`).
  - Keys: Esc, a, p, o (`document-list.component.ts:329-372`).
- **List state** lives in the URL query and in localStorage
  (`document-list-view.service.ts:350-356, 186-195`).

## b. Filters and saved views

- **The text box has a target selector:** title; title and content (the default); ASN; file type;
  advanced full-text; and "More like" (`filter-editor/filter-editor.component.ts:126-133, 200-224`).
  It waits 400 ms and at least 3 characters before searching (`:1289-1291`).
- **One dropdown per taxonomy** (tags, correspondent, type, storage path).
  - Each dropdown has a search box, and each item a count of documents **in the current result
    set**, from the `selection_data` returned with every list call
    (`document-list-view.service.ts:338, 348`). Items at zero are dimmed.
  - A "Not assigned" item sits at the top, and selected items sort first
    (`filterable-dropdown.component.ts:107-170`).
- **How values combine.**
  - Tags: All or Any. Clicking a selected tag again excludes it
    (`filterable-dropdown.component.ts:213-260`).
  - Single-value fields: Include or Exclude, plus "not assigned"
    (`filter-editor.component.ts:947-1025`).
  - Dropdowns AND together.
- **Dates:** created and added ranges, plus relative presets (week, month, quarter, year, this
  year and so on) (`components/common/dates-dropdown/dates-dropdown.component.ts:85-163`).
- **The custom-field query builder** has nested Any/All/Not groups, operators per field type, and
  a typed value input. A condition counts only when complete
  (`components/common/custom-fields-query-dropdown/`; `data/custom-field-query.ts:37-116`).
- **There is no chip row.** An active dropdown turns solid with a numbered "×" badge, and a global
  Reset appears only when something changed (`clearable-badge/*.html:1-11`;
  `filter-editor.component.ts:1185-1193`).
- **Saved views** keep the rules, sort, page size, display mode and columns
  (`data/saved-view.ts`).
  - A changed view shows `*` and offers "Save" (`document-list.component.ts:161-216`).
  - The sidebar lists views with live counts (`app-frame/app-frame.component.html:111-130`).

## c. Bulk editing

- **The bulk editor replaces the filter bar** while documents are selected, with the same
  dropdowns (`document-list.component.html:119-122`; `bulk-editor/bulk-editor.component.html:1-120`).
- **Three states per value:** on every selected document, on some, on none
  (`bulk-editor.component.ts:369-400`). Apply is enabled only on a change, and it sends a diff of
  values to add and to remove (`filterable-dropdown.component.ts:870-877`).
- **A confirmation names the exact change and N documents.** It is on by default
  (`bulk-editor.component.ts:515-756`; `data/ui-settings.ts:136-138`).
- **After applying:** the list reloads and the selection shrinks to what still matches
  (`bulk-editor.component.ts:324-336`).
- **There is no undo.** A per-document History tab shows the audit log, read-only
  (`document-detail/document-history/*.html:28-31`).

## d. Document detail

- **The details form** holds title, ASN, created date, correspondent, type, storage path, tags,
  and one typed input per custom field (`document-detail.component.html:177-286`).
  - Each select has a clear option, a "+" to create an item, and a "filter by this" button
    (`.ts:1795-1830`).
- **Saving is explicit:** Save, Save & close, Save & next and Discard, each enabled only when the
  form has changed (`.html:460-470`). The keys are Ctrl+S and Ctrl+Shift+S.
  - Only changed fields are sent (`.ts:1320-1331`).
  - Server validation errors are shown on each field.
  - Closing a changed document asks first (`services/open-documents.service.ts:132-160`).
- **Custom fields.**
  - A menu lists the fields the document does not yet have, and "Create new field" adds an
    empty row. Nothing is stored until Save (`docs/usage.md:770-776`).
  - The types are text, longtext, date, integer, float, monetary (a currency plus an amount),
    boolean, URL, document link (a typeahead) and select (`document-detail.component.html:200-286`;
    `input/monetary/monetary.component.ts:43-92`).
  - A field's type is fixed after creation.

## e. Inbox and review

- **An "Inbox tag"** is auto-assigned to every new document (`tag-edit-dialog.component.html:17`).
  The dashboard shows "Documents in inbox: N" (`statistics-widget/*.html:27-31`).
- **A document becomes reviewed when its inbox tag is removed:** by hand, or on save, if the
  setting is on (off by default; `data/ui-settings.ts:323-325`).
- **Save & next walks the filtered list** (`document-list-view.service.ts:633-656`).
- **The documented routine** (`docs/usage.md:1145-1162`): ASN, correspondent, type, title, check
  the date, then remove the inbox tag.

## f. Suggestions

- **Fetching.** Suggestions load automatically on open only while the document still has an inbox
  tag; otherwise they wait for the Suggest button (`document-detail.component.ts:955-971`;
  `suggestions-dropdown/*.html:3-19`).
- **Sources.** ML, AI, or both. They are merged as a union per field, and **the UI never says
  which source suggested what** (`data/document-suggestions.ts:21-49`).
- **Display.** A suggestion appears under each field as a link (`input/select/*.html:74-80`,
  `input/tags/*.html:64-71`). One click puts it in the form as an unsaved change.
- **There is no reject.** An ignored suggestion is never stored.
- **A suggested name that matches no existing item** appears only in the Suggest dropdown, and
  opens a prefilled create dialog (`suggestions-dropdown/*.html:33-50`).

## g. Taxonomy management

- **One page per kind,** each a paginated table with Name, Matching and Document count, plus
  Edit, Delete and "Filter documents (N)" on every row
  (`manage/document-attributes/management-list/*.html:1-151`).
  - The Matching column shows "Automatic", "None", or the algorithm with its pattern
    (`management-list.component.ts:139-151`).
- **The edit dialog** has the matching algorithm, and the pattern fields only for Any, All, Exact,
  Regex and Fuzzy (`edit-dialog/edit-dialog.component.ts:145-150`). Each algorithm comes with a
  one-line explanation (`data/matching-model.ts:12-48`).
- **Nested tags** go up to depth 5 (`docs/usage.md:114-122`).

## The five behaviours worth taking first

1. **Per-field inline suggestions, accepted by a click and kept only by Save.** Aura adds the one
   thing Paperless lacks: the source of every suggestion (rule, classifier, LLM), and an explicit
   reject, which is a negative label.
2. **The review loop:** a "to review" count, suggestions on unreviewed documents only, an explicit
   "reviewed" action, and Save & next. Reviewed documents are the only clean training labels.
3. **Taxonomy dropdowns.** Counts over the current results; Include, Exclude and "Not assigned";
   Any or All for tags; and a value that filters when clicked. They map one to one onto the
   agent's `document_search` filter arguments.
4. **Bulk edit.** Three states per value, a diff on Apply, a confirmation naming the change and N,
   and "all" meaning the filter minus its exclusions.
5. **The rule on the label.** A Matching column, the pattern shown only when the algorithm needs
   one, and "Filter documents (N)". "Auto" is the per-label opt-in for a classifier.

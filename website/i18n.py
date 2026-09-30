#!/usr/bin/env python3
"""Build the translated copies of the site from the English pages.

Danish was written by hand and stays that way. German, French and Spanish are
generated: this finds every translatable run of text in the English source,
swaps in the translation from i18n/<lang>.json, and rewrites the paths and the
language switcher for a subdirectory.

It edits raw bytes instead of re-serialising a parse tree, so indentation,
attribute order, comments and entities survive untouched. That is what keeps a
generated page reviewable in a diff and structurally identical to English — the
drift already visible between en/ and da/ becomes impossible by construction.

What is deliberately NOT translated:

    <code> <pre>       commands, paths and file names. Translating
                       `keepassxc-cli` would be a bug.
    <script> <style>   not prose.
    href / src         rewritten structurally, never translated.

Segments are keyed by a hash of the English text, not by position. Editing one
English sentence therefore invalidates exactly that one translation and leaves
the rest standing, and text repeated across pages — the nav, the footer — is
translated once and reused everywhere.

    python3 website/i18n.py extract         # English → i18n/segments.json
    python3 website/i18n.py check           # prove the rewriter is lossless
    python3 website/i18n.py build de fr es  # write website/<lang>/*.html
"""

import hashlib
import json
import pathlib
import re
import sys
from html.parser import HTMLParser

ROOT = pathlib.Path(__file__).resolve().parent
I18N = ROOT / "i18n"

# dir is the URL segment; name is what the language calls itself.
LANGS = {
    "en": {"dir": "", "label": "EN", "name": "English", "picker": "Language"},
    "da": {"dir": "da", "label": "DA", "name": "Dansk", "picker": "Sprog"},
    "de": {"dir": "de", "label": "DE", "name": "Deutsch", "picker": "Sprache"},
    "fr": {"dir": "fr", "label": "FR", "name": "Français", "picker": "Langue"},
    "es": {"dir": "es", "label": "ES", "name": "Español", "picker": "Idioma"},
}

# Flags are inlined rather than referenced from a <symbol> sprite: a <use> that
# points inside a collapsed <details> has been unreliable across browsers, and
# six small shapes cost about 1.5 kB a page. Emoji flags were not an option —
# Windows ships no flag glyphs and would render them as bare letter pairs.
#
# A flag is a country and a language is not, so each one is paired with its code
# and its own name; the flag is decoration, the text is the label.
FLAGS = {
    "en": '<rect width="21" height="15" fill="#012169"/>'
          '<path d="M0 0l21 15M21 0L0 15" stroke="#fff" stroke-width="3"/>'
          '<path d="M0 0l21 15M21 0L0 15" stroke="#C8102E" stroke-width="1.6"/>'
          '<path d="M10.5 0v15M0 7.5h21" stroke="#fff" stroke-width="5"/>'
          '<path d="M10.5 0v15M0 7.5h21" stroke="#C8102E" stroke-width="3"/>',
    "da": '<rect width="21" height="15" fill="#C8102E"/>'
          '<path d="M0 6.5h21v2H0zM6.5 0h2v15h-2z" fill="#fff"/>',
    "de": '<rect width="21" height="5" fill="#000"/>'
          '<rect y="5" width="21" height="5" fill="#D00"/>'
          '<rect y="10" width="21" height="5" fill="#FFCE00"/>',
    "fr": '<rect width="7" height="15" fill="#002395"/>'
          '<rect x="7" width="7" height="15" fill="#fff"/>'
          '<rect x="14" width="7" height="15" fill="#ED2939"/>',
    "es": '<rect width="21" height="15" fill="#AA151B"/>'
          '<rect y="3.75" width="21" height="7.5" fill="#F1BF00"/>',
}


def flag(code):
    return (f'<svg class="flag" viewBox="0 0 21 15" aria-hidden="true">'
            f'{FLAGS[code]}</svg>')
SOURCE = "en"
SITE = "https://deltasync.org"

SKIP_TAGS = {"script", "style", "code", "pre", "kbd", "samp"}
VOID = {"br", "hr", "img", "meta", "link", "input", "source", "area", "base", "col"}
ATTRS = ("alt", "aria-label")
# og:url and twitter:card are machine-facing; only prose belongs here.
META_KEYS = {"description", "og:title", "og:description",
             "twitter:title", "twitter:description"}


def key_of(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:10]


class Spans(HTMLParser):
    """Collect (start, end, raw_text) for every translatable span."""

    def __init__(self, raw):
        super().__init__(convert_charrefs=False)
        self.raw = raw
        self.line_start = [0]
        for i, ch in enumerate(raw):
            if ch == "\n":
                self.line_start.append(i + 1)
        self.stack = []
        self.spans = []
        self.run = None

    def pos(self):
        line, col = self.getpos()
        return self.line_start[line - 1] + col

    def flush(self):
        """Close the current text run at the current position."""
        if self.run is None:
            return
        start, self.run = self.run, None
        end = self.pos()
        chunk = self.raw[start:end]
        if not chunk.strip() or set(self.stack) & SKIP_TAGS:
            return
        # Leave the indentation out of the segment; a translator should never
        # have to reproduce whitespace.
        lead = len(chunk) - len(chunk.lstrip())
        tail = len(chunk) - len(chunk.rstrip())
        self.spans.append((start + lead, end - tail, chunk.strip()))

    # Entities arrive as their own events with convert_charrefs=False, so a
    # sentence containing &mdash; would otherwise be split into three segments.
    # Keeping one open run across them holds the sentence together.
    def handle_data(self, data):
        if self.run is None:
            self.run = self.pos()

    def handle_entityref(self, name):
        if self.run is None:
            self.run = self.pos()

    def handle_charref(self, name):
        if self.run is None:
            self.run = self.pos()

    def _attrs(self, tag, attrs):
        raw_tag = self.get_starttag_text() or ""
        base = self.pos()
        d = dict(attrs)
        wanted = list(ATTRS)
        if tag == "meta" and (d.get("name") in META_KEYS or d.get("property") in META_KEYS):
            wanted.append("content")
        for name in wanted:
            value = d.get(name)
            if not value:
                continue
            at = raw_tag.find(f'{name}="')
            if at < 0:
                continue
            s = base + at + len(name) + 2
            e = s + len(value)
            # Only take the span when the raw text agrees, so an entity inside
            # an attribute is left alone rather than mangled.
            if self.raw[s:e] == value:
                self.spans.append((s, e, value))

    def handle_starttag(self, tag, attrs):
        self.flush()
        self._attrs(tag, attrs)
        if tag not in VOID:
            self.stack.append(tag)

    def handle_startendtag(self, tag, attrs):
        self.flush()
        self._attrs(tag, attrs)

    def handle_endtag(self, tag):
        self.flush()
        if tag in self.stack:
            while self.stack and self.stack.pop() != tag:
                pass

    def handle_comment(self, data):
        self.flush()

    def handle_decl(self, decl):
        self.flush()

    def close(self):
        self.flush()
        super().close()


def spans_of(raw):
    p = Spans(raw)
    p.feed(raw)
    p.close()
    return sorted(set(p.spans))


def pages():
    return sorted(p.name for p in ROOT.glob("*.html"))


# The switcher labels and the footer's language names are rebuilt structurally,
# so they must not also appear as prose for someone to translate.
STRUCTURAL = {l["label"] for l in LANGS.values()} | {l["name"] for l in LANGS.values()}


def translatable(text):
    """Is this span prose, or just the punctuation between two inline tags?

    Splitting on inline elements leaves fragments like ".", ")." and the
    footer's "&nbsp;·&nbsp;". They carry no words, they are identical in every
    language, and asking anyone to translate them invites a typo in markup.
    """
    # Entity names are letters too: "&nbsp;·&nbsp;" would otherwise look like
    # prose because of the "nbsp". Strip entities before asking.
    bare = re.sub(r"&(?:#\d+|#[xX][0-9a-fA-F]+|\w+);", "", text)
    return text not in STRUCTURAL and any(ch.isalpha() for ch in bare)


def apply_text(raw, spans, table, page=None):
    """Substitute translations into raw, longest-lived key first.

    Keying by content is what lets the nav be translated once for the whole
    site, but it cannot tell two uses of the same English apart — and a
    function word is exactly where that bites. "the" is "der" before a
    masculine noun and "des" in a genitive, and both occur on one page.

    So a language file may carry an "@fix" section, keyed by page and by which
    occurrence of that text it is:

        "@fix": {"deploy-server.html": {"the#2": "des"}}

    Counting occurrences of the text itself, rather than spans on the page,
    keeps a fix pointing at the same words when unrelated markup moves.
    """
    fixes = (table.get("@fix") or {}).get(page or "", {})
    seen = {}
    out, prev, missing = [], 0, 0
    for s, e, text in spans:
        out.append(raw[prev:s])
        seen[text] = seen.get(text, 0) + 1
        t = fixes.get(f"{text}#{seen[text]}")
        if t is None:
            t = table.get(key_of(text))
        if t is None:
            missing += 1 if translatable(text) else 0
            t = text
        # Where English had a word and the target language has none, the space
        # the markup put between them is left stranded: ".kdbx file" becomes
        # ".kdbx -Datei", and "the guide first." becomes "die Anleitung ." So a
        # translation that opens with a hyphen (a compound) or with closing
        # punctuation closes up with what precedes it. An em dash is prose and
        # keeps its space, as does an opening bracket.
        if t[:1] in "-.,;:!?)" and out and out[-1][-1:] in " \t":
            out[-1] = out[-1].rstrip(" \t")
        out.append(t)
        prev = e
    out.append(raw[prev:])
    return "".join(out), missing


def link_to(page, from_lang, to_lang):
    fd, td = LANGS[from_lang]["dir"], LANGS[to_lang]["dir"]
    if fd == td:
        return page
    return f"{'../' if fd else ''}{td + '/' if td else ''}{page}"


def abs_url(page, lang):
    d = LANGS[lang]["dir"]
    base = f"{SITE}/{d + '/' if d else ''}"
    return base if page == "index.html" else base + page


def rewrite_structure(text, page, lang, paths=True):
    """Point a page at its own language: switcher, alternates, footer, paths.

    `paths` is off when refreshing a page that already lives in its own
    directory — the hand-written Danish ones — because their asset links have
    already climbed out and would otherwise climb twice.
    """
    ind = "    "

    # 1. Paths climb out of the subdirectory.
    if paths and LANGS[lang]["dir"]:
        text = re.sub(r'((?:href|src|srcset)=")(assets/)', r'\1../\2', text)
        text = re.sub(r'(href=")(style\.css")', r'\1../\2', text)

    # 2. The document's own language.
    text = re.sub(r'(<html\b[^>]*\blang=")[^"]*(")', rf'\g<1>{lang}\g<2>', text, count=1)

    # 3. hreflang alternates: one contiguous block, rebuilt whole.
    alts = "\n".join(
        f'{ind}<link rel="alternate" hreflang="{c}" href="{abs_url(page, c)}">'
        for c in LANGS
    ) + f'\n{ind}<link rel="alternate" hreflang="x-default" href="{abs_url(page, SOURCE)}">'
    text = re.sub(
        r'[ \t]*<link rel="alternate" hreflang="[^"]*" href="[^"]*">\n'
        r'(?:[ \t]*<link rel="alternate" hreflang="[^"]*" href="[^"]*">\n)*',
        alts + "\n", text, count=1)

    # 4. The picker, page-aware: every language points at this same page. It is
    #    a <details> disclosure so it needs no JavaScript — the site ships none,
    #    and faq.html already uses the same element for its accordion.
    def switch(m):
        pad = m.group(1) if m.group(1) is not None else m.group(2)
        rows = "".join(
            f'\n{pad}{ind}{ind}<a href="{link_to(page, lang, c)}" hreflang="{c}"'
            f'{" aria-current=\"true\"" if c == lang else ""}>'
            f'{flag(c)}<span class="code">{LANGS[c]["label"]}</span>'
            f'<span class="name">{LANGS[c]["name"]}</span></a>'
            for c in LANGS)
        return (f'{pad}<details class="lang-switch">'
                f'\n{pad}{ind}<summary aria-label="{LANGS[lang]["picker"]}">'
                f'{flag(lang)}<span class="code">{LANGS[lang]["label"]}</span></summary>'
                f'\n{pad}{ind}<div class="lang-menu">{rows}'
                f'\n{pad}{ind}</div>'
                f'\n{pad}</details>')

    # Matches the old <span> form on the first pass and the <details> form on
    # every pass after, so re-running is safe.
    text = re.sub(r'([ \t]*)<span class="lang-switch">.*?</span>'
                  r'|([ \t]*)<details class="lang-switch">.*?</details>',
                  switch, text, count=1, flags=re.S)

    # 5. The footer lists every other language by its own name. In the English
    #    source that is the single "Dansk" link.
    def footer(m):
        pad = m.group(1)
        # Matches the separator the hand-written pages already use, so a
        # generated footer is indistinguishable from theirs in a diff.
        sep = f'\n{pad}&nbsp;·&nbsp;\n{pad}'
        return pad + sep.join(
            f'<a href="{link_to(page, lang, c)}">{LANGS[c]["name"]}</a>'
            for c in LANGS if c != lang)

    names = "|".join(re.escape(l["name"]) for l in LANGS.values())
    text = re.sub(rf'([ \t]*)<a href="[^"]*">(?:{names})</a>'
                  rf'(?:\s*&nbsp;·&nbsp;\s*<a href="[^"]*">(?:{names})</a>)*',
                  footer, text, count=1)
    return text


def cmd_extract():
    I18N.mkdir(exist_ok=True)
    segments, per_page = {}, {}
    for name in pages():
        raw = (ROOT / name).read_text(encoding="utf-8")
        keys = []
        for _, _, text in spans_of(raw):
            if not translatable(text):
                continue
            segments[key_of(text)] = text
            keys.append(key_of(text))
        per_page[name] = keys
    (I18N / "segments.json").write_text(
        json.dumps(segments, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    total = sum(len(v) for v in per_page.values())
    words = sum(len(t.split()) for t in segments.values())
    print(f"{len(pages())} pages, {total} spans, {len(segments)} unique, ~{words} words to translate")
    for name in pages():
        print(f"  {name:<24}{len(per_page[name]):>4} spans")


def cmd_check():
    """Replacing every span with itself must reproduce the file byte for byte."""
    bad = 0
    for name in pages():
        raw = (ROOT / name).read_text(encoding="utf-8")
        spans = spans_of(raw)
        same = {key_of(t): t for _, _, t in spans}
        out, _ = apply_text(raw, spans, same)
        ok = out == raw
        bad += 0 if ok else 1
        print(f"  {name:<24}{len(spans):>4} spans   {'lossless' if ok else 'CHANGED THE FILE'}")
    return 1 if bad else 0


def cmd_build(langs):
    for lang in langs:
        if lang not in LANGS or lang == SOURCE:
            print(f"unknown or non-buildable language: {lang}", file=sys.stderr)
            return 2
        table_path = I18N / f"{lang}.json"
        table = json.loads(table_path.read_text(encoding="utf-8")) if table_path.exists() else {}
        out_dir = ROOT / LANGS[lang]["dir"]
        out_dir.mkdir(exist_ok=True)
        gaps = 0
        for name in pages():
            raw = (ROOT / name).read_text(encoding="utf-8")
            text, missing = apply_text(raw, spans_of(raw), table, page=name)
            text = rewrite_structure(text, name, lang)
            (out_dir / name).write_text(text, encoding="utf-8")
            gaps += missing
        note = f", {gaps} spans still English" if gaps else ""
        print(f"  {lang}: wrote {len(pages())} pages to {out_dir.name}/{note}")
    return 0


def cmd_nav(langs):
    """Refresh switcher, alternates and footer on the hand-written languages.

    English and Danish are not generated, so adding a language leaves their
    switchers behind. This rewrites only those three regions in place; the prose
    and the markup around them are untouched, which a diff should confirm.
    """
    for lang in langs:
        out_dir = ROOT / LANGS[lang]["dir"]
        for name in pages():
            f = out_dir / name
            if not f.exists():
                continue
            raw = f.read_text(encoding="utf-8")
            f.write_text(rewrite_structure(raw, name, lang, paths=False), encoding="utf-8")
        print(f"  {lang}: refreshed {len(pages())} pages in {LANGS[lang]['dir'] or '.'}/")
    return 0


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    if cmd == "extract":
        cmd_extract(); return 0
    if cmd == "check":
        return cmd_check()
    if cmd == "build":
        return cmd_build(sys.argv[2:] or list(LANGS))
    if cmd == "nav":
        return cmd_nav(sys.argv[2:] or [SOURCE, "da"])
    print(__doc__.strip().splitlines()[-4:][0], file=sys.stderr)
    print("commands: extract | check | build <lang>...", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

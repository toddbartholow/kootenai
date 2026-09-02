/**
 * Pinned to jsdom; the rest of the suite runs on happy-dom (vitest.config.ts).
 *
 * DOMPurify's README states happy-dom "are not considered safe" and that
 * combining the two "will likely lead to XSS"; upstream tests on jsdom and
 * Chromium only. That is not theoretical here. Since 3.4.8 DOMPurify reads tag
 * names through the Node.prototype `nodeName` getter it captures at init
 * (anti-clobbering hardening). happy-dom also defines `nodeName` on
 * Element.prototype, so the captured getter returns "" for every element:
 * every tag reads as disallowed, the first node is dropped whatever it was,
 * and iteration stops there. The result is a sanitizer that is inert rather
 * than merely lossy -- under happy-dom,
 *
 *   sanitize('<p>safe</p><script>alert(1)</script>')
 *     === 'safe<script>alert(1)</script>'
 *
 * with the script tag surviving verbatim. `<img onerror>`, `<iframe
 * src="javascript:">` and friends likewise pass through untouched whenever
 * they are not the first node.
 *
 * Verified on happy-dom 20.10.6 (the pinned version) and 20.12.0; dompurify
 * 3.4.7 predates the hardening and is unaffected. jsdom 30 and real Chromium
 * both sanitize correctly with this component's exact config.
 *
 * InstructionsPanel.vue is the only DOMPurify consumer in src/, so only this
 * file needs the override. That means it runs on a DOM the other 65 files do
 * not: 9 files / 152 tests fail suite-wide under jsdom (PrimeVue Select's
 * mounted hook). Keep this file to pure renderMarkdown assertions -- do not
 * add mount() here.
 *
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest'
import { renderMarkdown } from './InstructionsPanel.vue'

describe('renderMarkdown', () => {
  describe('basic markdown rendering', () => {
    it('should render headers', () => {
      expect(renderMarkdown('# Heading 1')).toContain('<h1>Heading 1</h1>')
      expect(renderMarkdown('## Heading 2')).toContain('<h2>Heading 2</h2>')
      expect(renderMarkdown('### Heading 3')).toContain('<h3>Heading 3</h3>')
    })

    it('should render bold text', () => {
      const result = renderMarkdown('**bold text**')
      expect(result).toContain('<strong>bold text</strong>')
    })

    it('should render italic text', () => {
      const result = renderMarkdown('*italic text*')
      expect(result).toContain('<em>italic text</em>')
    })

    it('should render inline code', () => {
      const result = renderMarkdown('Use `npm install` to install')
      expect(result).toContain('<code>npm install</code>')
    })

    it('should render code blocks', () => {
      const result = renderMarkdown('```bash\nnpm install\n```')
      expect(result).toContain('<pre>')
      expect(result).toContain('<code')
      expect(result).toContain('npm install')
    })

    it('should render links with target blank', () => {
      const result = renderMarkdown('[Click here](https://example.com)')
      expect(result).toContain('href="https://example.com"')
      expect(result).toContain('target="_blank"')
      expect(result).toContain('>Click here</a>')
    })

    it('should render unordered lists', () => {
      const result = renderMarkdown('- Item 1\n- Item 2')
      expect(result).toContain('<li>Item 1</li>')
      expect(result).toContain('<li>Item 2</li>')
      expect(result).toContain('<ul>')
    })

    it('should handle empty input', () => {
      expect(renderMarkdown('')).toBe('')
    })

    it('should handle null-like input', () => {
      expect(renderMarkdown(null as unknown as string)).toBe('')
      expect(renderMarkdown(undefined as unknown as string)).toBe('')
    })
  })

  describe('XSS prevention with DOMPurify', () => {
    it('should remove script tags', () => {
      const malicious = '<script>alert("XSS")</script>Hello'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('<script>')
      expect(result).not.toContain('alert')
      expect(result).toContain('Hello')
    })

    it('should remove onclick handlers', () => {
      const malicious = '<div onclick="alert(1)">Click me</div>'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('onclick')
      expect(result).toContain('Click me')
    })

    it('should remove onerror handlers', () => {
      const malicious = '<img src="x" onerror="alert(1)">'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('onerror')
      // img tags are not in allowed list, so it should be removed
      expect(result).not.toContain('<img')
    })

    it('should remove javascript: URLs', () => {
      const malicious = '[Click](javascript:alert(1))'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('javascript:')
    })

    it('should remove data: URLs', () => {
      const malicious = '[Click](data:text/html,<script>alert(1)</script>)'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('data:')
    })

    it('should remove iframe tags', () => {
      const malicious = '<iframe src="https://evil.com"></iframe>Content'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('<iframe')
      expect(result).toContain('Content')
    })

    it('should remove form tags and action attributes', () => {
      const malicious =
        '<form action="https://evil.com" method="post"><input type="hidden" name="token" value="secret"></form>'
      const result = renderMarkdown(malicious)
      // Form tag must be removed to prevent form submission
      expect(result).not.toContain('<form')
      expect(result).not.toContain('action=')
      // Note: DOMPurify may keep orphaned input as it's harmless without a form
    })

    it('should remove SVG with embedded scripts', () => {
      const malicious = '<svg onload="alert(1)"><script>alert(2)</script></svg>'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('<svg')
      expect(result).not.toContain('<script')
      expect(result).not.toContain('onload')
    })

    it('should remove style tags', () => {
      const malicious = '<style>body{background:url("javascript:alert(1)")}</style>'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('<style')
    })

    it('should remove event handlers from allowed tags', () => {
      const malicious = '<a href="#" onmouseover="alert(1)">Link</a>'
      const result = renderMarkdown(malicious)
      expect(result).not.toContain('onmouseover')
      expect(result).toContain('>Link</a>')
    })

    it('should preserve safe markdown content', () => {
      const safe = '# Title\n\nThis is **bold** and *italic* text with `code`.'
      const result = renderMarkdown(safe)
      expect(result).toContain('<h1>Title</h1>')
      expect(result).toContain('<strong>bold</strong>')
      expect(result).toContain('<em>italic</em>')
      expect(result).toContain('<code>code</code>')
    })

    it('should handle mixed safe and malicious content', () => {
      const mixed = '# Safe Title\n\n<script>evil()</script>\n\n**Bold text**'
      const result = renderMarkdown(mixed)
      expect(result).toContain('<h1>Safe Title</h1>')
      expect(result).toContain('<strong>Bold text</strong>')
      expect(result).not.toContain('<script>')
      expect(result).not.toContain('evil')
    })

    it('should only allow whitelisted attributes', () => {
      // href, target, rel, class are allowed
      const result = renderMarkdown('[Link](https://example.com)')
      expect(result).toContain('href=')
      expect(result).toContain('target=')
      expect(result).toContain('rel=')

      // style and other attributes should be removed
      const malicious = '<div style="color:red" data-evil="yes">Content</div>'
      const maliciousResult = renderMarkdown(malicious)
      expect(maliciousResult).not.toContain('style=')
      expect(maliciousResult).not.toContain('data-evil')
    })
  })

  describe('edge cases', () => {
    it('should handle deeply nested tags', () => {
      const nested = '<div><div><div><script>alert(1)</script></div></div></div>'
      const result = renderMarkdown(nested)
      expect(result).not.toContain('<script>')
    })

    it('should handle malformed HTML', () => {
      const malformed = '<script>alert(1)<div>Content</script></div>'
      const result = renderMarkdown(malformed)
      expect(result).not.toContain('<script>')
    })

    it('should handle encoded entities that could be XSS', () => {
      const encoded = '&lt;script&gt;alert(1)&lt;/script&gt;'
      const result = renderMarkdown(encoded)
      // Encoded entities should remain encoded, not execute
      expect(result).not.toMatch(/<script>/i)
    })

    it('should handle very long input', () => {
      const longText = '# Title\n\n' + 'Paragraph. '.repeat(1000)
      const result = renderMarkdown(longText)
      expect(result).toContain('<h1>Title</h1>')
      expect(result.length).toBeGreaterThan(0)
    })
  })
})

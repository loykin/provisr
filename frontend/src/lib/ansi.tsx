import Anser from 'anser'
import type { CSSProperties } from 'react'

// Parses ANSI escape codes (color, bold, underline, ...) out of raw process
// output and renders them as styled spans. Managed processes often force
// color output even when not attached to a TTY, so without this the escape
// sequences show up as literal garbage (e.g. "^[[32m") in the browser.
export function renderAnsiLine(text: string) {
  const chunks = Anser.ansiToJson(text, { use_classes: false, remove_empty: true })
  return chunks.map((chunk, i) => {
    const style: CSSProperties = {}
    if (chunk.fg) style.color = `rgb(${chunk.fg})`
    if (chunk.bg) style.backgroundColor = `rgb(${chunk.bg})`
    if (chunk.decorations.includes('bold')) style.fontWeight = 'bold'
    if (chunk.decorations.includes('italic')) style.fontStyle = 'italic'
    if (chunk.decorations.includes('underline')) style.textDecoration = 'underline'
    if (chunk.decorations.includes('strikethrough')) style.textDecoration = 'line-through'
    if (chunk.decorations.includes('dim')) style.opacity = 0.6
    return (
      <span key={i} style={style}>
        {chunk.content}
      </span>
    )
  })
}

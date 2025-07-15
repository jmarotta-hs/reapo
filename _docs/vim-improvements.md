# Vim Textarea - Known Limitations and Improvements

This document tracks the current state of vim functionality in the textarea component and areas for future improvement.

## ✅ Implemented and Working

### Basic Movement
- `h`, `j`, `k`, `l` - Basic directional movement
- Arrow keys - Alternative directional movement
- Proper scrolling integration for all movements

### Word Movement
- `w` - Move to beginning of next word
- `W` - Move to beginning of next WORD (whitespace-separated)
- `b` - Move to beginning of previous word
- `B` - Move to beginning of previous WORD
- `e` - Move to end of current/next word ✅ **Recently Fixed**
- `E` - Move to end of current/next WORD ✅ **Recently Fixed**

### Line Movement
- `0` - Move to beginning of line
- `^` - Move to first non-whitespace character
- `_` - Move to first non-whitespace character (alternative)
- `$` - Move to end of line

### Document Navigation
- `G` - Go to last line (or numbered line with count)
- `gg` - Go to first line ✅ **Recently Fixed scrolling**

### Text Operations
- `dd` - Delete line
- `yy` - Yank (copy) line
- `p` - Paste after cursor
- `P` - Paste before cursor
- `u` - Undo
- `r` - Replace single character
- `D` - Delete to end of line
- `C` - Change to end of line
- `Y` - Yank to end of line

### Mode Support
- Normal mode
- Insert mode (`i`, `I`, `a`, `A`, `o`, `O`)
- Visual mode (`v`)
- Proper cursor positioning per mode

### Advanced Features
- Operator-motion commands (`d{motion}`, `y{motion}`, `c{motion}`)
- Count prefixes for commands (e.g., `3w`, `5dd`)
- Undo/redo system with history
- Clipboard operations
- Dynamic scrolling with viewport management

## ⚠️ Known Limitations

### Find/Search Commands (Not Implemented)
These commands are stubbed out and need full implementation:

- `f{char}` - Find character forward on line
- `F{char}` - Find character backward on line  
- `t{char}` - Till character forward (stop before)
- `T{char}` - Till character backward (stop before)
- `;` - Repeat last find command
- `,` - Repeat last find command in opposite direction

**Implementation Notes:**
- Requires state management to capture the target character
- Need to store last find operation for repeat commands
- Should highlight/indicate found characters

### Multi-Key G Commands
Currently only `gg` is implemented. Missing:
- `g0` - Go to first character of screen line
- `g$` - Go to last character of screen line
- `gj` - Move down one screen line
- `gk` - Move up one screen line

**Implementation Notes:**
- Requires state machine for multi-key command sequences
- Need to distinguish between `g` + next key vs single `G`

### Advanced Text Objects
Not implemented:
- `iw` - Inner word
- `aw` - A word  
- `ip` - Inner paragraph
- `ap` - A paragraph
- `i"` - Inner quotes
- `a"` - A quotes (including quotes)

### Search and Replace
Not implemented:
- `/pattern` - Search forward
- `?pattern` - Search backward
- `n` - Next search result
- `N` - Previous search result
- `*` - Search for word under cursor
- `#` - Search for word under cursor backward

### Advanced Navigation
Not implemented:
- `%` - Jump to matching bracket/paren
- `(` - Sentence backward
- `)` - Sentence forward
- `{` - Paragraph backward  
- `}` - Paragraph forward

### Marks and Jumps
Not implemented:
- `m{letter}` - Set mark
- `'{letter}` - Jump to mark
- `` ` `` - Jump to exact mark position
- `''` - Jump to previous position

## 🔧 Recent Fixes

### Fixed in Current Session
1. **Word End Movement (`e`/`E`)** - Completely rewrote `nextWordEnd` function to properly:
   - Move to end of current word when in middle
   - Jump to end of next word when already at word end
   - Handle line boundaries correctly

2. **Scrolling Integration** - Added `adjustScroll()` calls to:
   - All movement commands in Normal/Insert/Visual modes
   - `SetHeight()` function to fix insert mode scrolling issues
   - `handleGCommand()` for proper `gg` scrolling

3. **Insert Mode Scrolling** - Fixed issue where first line disappeared when creating newlines in insert mode

## 🚀 Suggested Implementation Priority

### High Priority
1. **Find Commands (`f`, `t`, etc.)** - Very commonly used in vim
2. **Search (`/`, `?`, `n`, `N`)** - Essential for text navigation
3. **Text Objects (`iw`, `aw`)** - Powerful for precise text selection

### Medium Priority  
1. **Multi-key G commands** - Useful for screen line navigation
2. **Bracket matching (`%`)** - Important for code editing
3. **Word search (`*`, `#`)** - Quick way to find identifier usage

### Low Priority
1. **Marks and jumps** - Advanced navigation features
2. **Sentence/paragraph movement** - Less common in code editing
3. **Advanced text objects** - Nice to have for complex editing

## Implementation Notes

### State Management Considerations
- Find commands need character input capture
- Multi-key commands need state machines
- Search needs pattern storage and result highlighting

### Performance Considerations
- Search operations should be optimized for large files
- Text object detection should be cached when possible
- Scrolling calculations should be efficient

### Integration Points
- Some features may need integration with the main TUI
- Search highlighting may require view layer changes
- Advanced navigation might need buffer management
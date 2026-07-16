import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react'
import './App.css'
import { Compile, CreateDemoProject, ExportPDF, LocateBlock, OpenProject, SaveBlock, SaveSource } from '../bindings/resumestudio/app'
import PDFPreview, { type PDFLocation } from './PDFPreview'

type Block = { id:string; type:string; title:string; file:string; startLine:number; endLine:number; content:string }
type Project = { path:string; name:string; mainFile:string; source:string; blocks:Block[] }
type Diagnostic = { severity:string; message:string; file?:string; line?:number }
type CompileResult = { revision:number; success:boolean; stale:boolean; engine:string; durationMs:number; pdfBase64?:string; diagnostics:Diagnostic[] }
type Mode = 'focus' | 'source'
type PaneSizes = [number, number, number]

const defaultPaneSizes: PaneSizes = [.21, .34, .45]
const minimumPaneWidths: PaneSizes = [180, 320, 360]

function constrainPaneSizes(sizes: PaneSizes, available: number): PaneSizes {
  if (available <= minimumPaneWidths[0] + minimumPaneWidths[1] + minimumPaneWidths[2]) return [...defaultPaneSizes]
  const widths = sizes.map(size => size * available) as PaneSizes
  for (let index = 0; index < widths.length; index++) {
    if (widths[index] >= minimumPaneWidths[index]) continue
    const deficit = minimumPaneWidths[index] - widths[index]
    widths[index] = minimumPaneWidths[index]
    const capacities = widths.map((width, donor) => donor === index ? 0 : Math.max(0, width - minimumPaneWidths[donor]))
    const totalCapacity = capacities.reduce((sum, capacity) => sum + capacity, 0)
    if (totalCapacity > 0) widths.forEach((width, donor) => { widths[donor] = width - deficit * capacities[donor] / totalCapacity })
  }
  return [widths[0] / available, widths[1] / available, widths[2] / available]
}

function loadPaneSizes(): PaneSizes {
  try {
    const value = JSON.parse(localStorage.getItem('resume-studio-pane-sizes') || '')
    if (Array.isArray(value) && value.length === 3 && value.every(item => typeof item === 'number' && item > 0)) {
      const total = value[0] + value[1] + value[2]
      return [value[0] / total, value[1] / total, value[2] / total]
    }
  } catch { /* use defaults */ }
  return [...defaultPaneSizes]
}

function normalizeProject(next: Project): Project { return { ...next, blocks: next.blocks || [] } }

const typeLabels: Record<string, string> = { experience:'工作经历', project:'项目经历', education:'教育经历', skills:'技能', summary:'个人简介', section:'其他' }

function Icon({ name }: { name:'file'|'folder'|'play'|'save'|'download'|'code'|'focus'|'pdf'|'check'|'alert' }) {
  const paths: Record<string, JSX.Element> = {
    file:<><path d="M14 2H6a2 2 0 0 0-2 2v16h16V8z"/><path d="M14 2v6h6"/></>,
    folder:<><path d="M3 7h6l2 2h10v10H3z"/><path d="M3 7V5h6l2 2"/></>,
    play:<path d="m8 5 11 7-11 7z"/>, save:<><path d="M5 3h12l2 2v16H5z"/><path d="M8 3v6h8V3M8 17h8"/></>, download:<><path d="M12 3v12M7 10l5 5 5-5"/><path d="M5 21h14"/></>,
    code:<><path d="m8 9-4 3 4 3M16 9l4 3-4 3M14 5l-4 14"/></>, focus:<><circle cx="12" cy="12" r="3"/><path d="M3 9V4h5M21 9V4h-5M3 15v5h5M21 15v5h-5"/></>,
    pdf:<><path d="M6 2h9l4 4v16H6z"/><path d="M14 2v5h5M9 16h6M9 12h6"/></>, check:<path d="m5 12 4 4L19 6"/>, alert:<><path d="M12 3 2 21h20z"/><path d="M12 9v5M12 18h.01"/></>
  }
  return <svg viewBox="0 0 24 24" aria-hidden="true">{paths[name]}</svg>
}

function App() {
  const [project, setProject] = useState<Project | null>(null)
  const [activeID, setActiveID] = useState('')
  const [mode, setMode] = useState<Mode>('focus')
  const [draft, setDraft] = useState('')
  const [savedDraft, setSavedDraft] = useState('')
  const [compiling, setCompiling] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [compileResult, setCompileResult] = useState<CompileResult | null>(null)
  const [pdfURL, setPdfURL] = useState('')
  const [pdfFocus, setPdfFocus] = useState<PDFLocation | null>(null)
  const [paneSizes, setPaneSizes] = useState<PaneSizes>(loadPaneSizes)
  const [message, setMessage] = useState('正在准备示例项目…')
  const saveTimer = useRef<number>()
  const requestRef = useRef(0)
  const focusRequestRef = useRef(0)
  const workspaceRef = useRef<HTMLElement>(null)

  const active = useMemo(() => project?.blocks.find(b => b.id === activeID), [project, activeID])

  useEffect(() => { CreateDemoProject().then(next => loadState(next as Project)).catch(showError) }, [])
  useEffect(() => () => { if (pdfURL) URL.revokeObjectURL(pdfURL) }, [pdfURL])
  useEffect(() => { localStorage.setItem('resume-studio-pane-sizes', JSON.stringify(paneSizes)) }, [paneSizes])
  useEffect(() => {
    const workspace = workspaceRef.current
    if (!workspace) return
    const observer = new ResizeObserver(entries => {
      const available = entries[0].contentRect.width - 12
      setPaneSizes(current => {
        const next = constrainPaneSizes(current, available)
        return next.every((value, index) => Math.abs(value - current[index]) < .0001) ? current : next
      })
    })
    observer.observe(workspace)
    return () => observer.disconnect()
  }, [])

  function resizeAt(splitter: 0 | 1, boundary: number, available: number) {
    const minimums = minimumPaneWidths.map(width => width / available)
    const clamp = (value: number, low: number, high: number) => Math.min(high, Math.max(low, value))
    setPaneSizes(current => {
      if (splitter === 0) {
        const pair = current[0] + current[1]
        const first = clamp(boundary, minimums[0], pair - minimums[1])
        return [first, pair - first, current[2]]
      }
      const firstTwo = clamp(boundary, current[0] + minimums[1], 1 - minimums[2])
      return [current[0], firstTwo - current[0], 1 - firstTwo]
    })
  }

  function beginResize(splitter: 0 | 1, event: ReactPointerEvent<HTMLDivElement>) {
    const workspace = workspaceRef.current
    if (!workspace) return
    event.preventDefault()
    const rect = workspace.getBoundingClientRect()
    const available = rect.width - 12
    const offset = splitter === 0 ? 3 : 9
    const move = (pointer: PointerEvent) => resizeAt(splitter, (pointer.clientX - rect.left - offset) / available, available)
    const finish = () => {
      document.body.classList.remove('pane-resizing')
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', finish)
      window.removeEventListener('pointercancel', finish)
    }
    document.body.classList.add('pane-resizing')
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', finish)
    window.addEventListener('pointercancel', finish)
  }

  function nudgeSplitter(splitter: 0 | 1, event: ReactKeyboardEvent<HTMLDivElement>) {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    const workspace = workspaceRef.current
    if (!workspace) return
    event.preventDefault()
    const available = workspace.getBoundingClientRect().width - 12
    const currentBoundary = splitter === 0 ? paneSizes[0] : paneSizes[0] + paneSizes[1]
    resizeAt(splitter, currentBoundary + (event.key === 'ArrowRight' ? .02 : -.02), available)
  }

  function loadState(next: Project, preferredID?: string) {
    const normalized = normalizeProject(next)
	setProject(normalized)
	const id = preferredID && normalized.blocks.some(b => b.id === preferredID) ? preferredID : normalized.blocks[0]?.id || ''
	setActiveID(id)
	const value = mode === 'source' ? normalized.source : normalized.blocks.find(b => b.id === id)?.content || ''
	setDraft(value); setSavedDraft(value)
	setMessage(`${normalized.name} · ${normalized.blocks.length} 个结构化段落`)
	setPdfFocus(null)
	void runCompile(normalized.path, id)
  }

  async function selectBlock(block: Block) {
    const next = draft !== savedDraft ? await saveNow() : project
    const fresh = next?.blocks.find(b => b.id === block.id) || block
    setMode('focus'); setActiveID(fresh.id); setDraft(fresh.content); setSavedDraft(fresh.content)
    if (next?.path) void focusPreview(next.path, fresh.id)
  }

  async function switchMode(nextMode: Mode) {
    if (!project || nextMode === mode) return
    const next = draft !== savedDraft ? await saveNow() : project
    if (!next) return
    setMode(nextMode)
    const value = nextMode === 'source' ? next.source : next.blocks.find(b => b.id === activeID)?.content || ''
    setDraft(value); setSavedDraft(value)
    if (nextMode === 'focus' && activeID) void focusPreview(next.path, activeID)
    else setPdfFocus(null)
  }

  function updateDraft(value: string) {
    setDraft(value)
    window.clearTimeout(saveTimer.current)
    saveTimer.current = window.setTimeout(() => void saveNow(value), 800)
  }

  async function saveNow(value = draft): Promise<Project | undefined> {
    if (!project) return
    if (value === savedDraft) return project
    window.clearTimeout(saveTimer.current)
    setMessage('正在保存…')
    try {
      const saved = mode === 'source'
        ? await SaveSource(project.path, value) as Project
        : await SaveBlock(project.path, activeID, value) as Project
	  const next = normalizeProject(saved)
      const id = activeID
      setProject(next); setSavedDraft(value); setMessage('已保存，正在更新预览…')
      if (mode === 'focus') {
        const fresh = next.blocks.find(b => b.id === id)
        if (fresh) { setDraft(fresh.content); setSavedDraft(fresh.content) }
      }
      await runCompile(next.path, activeID)
      return next
    } catch (error) { showError(error) }
  }

  async function runCompile(path = project?.path, focusID = activeID) {
    if (!path) return
    const request = ++requestRef.current
    setCompiling(true)
    try {
	  const raw = await Compile(path) as CompileResult
	  const result = { ...raw, diagnostics: raw.diagnostics || [] }
      if (request !== requestRef.current || result.stale) return
      setCompileResult(result)
      if (result.pdfBase64) {
        const binary = atob(result.pdfBase64), bytes = new Uint8Array(binary.length)
        for (let i=0; i<binary.length; i++) bytes[i] = binary.charCodeAt(i)
        const url = URL.createObjectURL(new Blob([bytes], {type:'application/pdf'}))
        setPdfURL(old => { if (old) URL.revokeObjectURL(old); return url })
      }
      if (result.pdfBase64 && focusID && mode === 'focus') void focusPreview(path, focusID)
      setMessage(result.success ? `编译完成 · ${result.engine} · ${result.durationMs}ms` : '编译失败 · 已保留上一次成功预览')
    } catch (error) { showError(error) }
    finally { if (request === requestRef.current) setCompiling(false) }
  }

  async function focusPreview(path: string, blockID: string) {
    const request = ++focusRequestRef.current
    try {
      const location = await LocateBlock(path, blockID) as PDFLocation
      if (request === focusRequestRef.current) setPdfFocus(location)
    } catch {
      if (request === focusRequestRef.current) setPdfFocus(null)
    }
  }

  async function chooseProject() {
    try { const next = await OpenProject() as Project; if (next?.path) loadState(next) } catch (error) { showError(error) }
  }

  async function exportPDF() {
    if (!project) return
    setExporting(true)
    try {
      const current = draft !== savedDraft ? await saveNow() : project
      if (!current) return
      const destination = await ExportPDF(current.path)
      if (destination) setMessage(`PDF 已导出 · ${destination}`)
    } catch (error) { showError(error) }
    finally { setExporting(false) }
  }

  function showError(error: unknown) { setMessage(error instanceof Error ? error.message : String(error)); setCompiling(false) }

  const dirty = draft !== savedDraft
  return <main className="app-shell">
    <header className="topbar">
      <div className="brand"><span className="brand-mark">R</span><div><strong>Resume Studio</strong><small>结构化 LaTeX 简历工作台</small></div></div>
      <div className="project-pill"><span className="pulse"/>{message}</div>
      <div className="top-actions">
        <button className="button ghost" onClick={chooseProject}><Icon name="folder"/>打开项目</button>
        <button className="button" disabled={!dirty} onClick={() => void saveNow()}><Icon name="save"/>{dirty ? '保存' : '已保存'}</button>
        <button className="button primary" disabled={compiling || !project} onClick={() => void runCompile()}><Icon name="play"/>{compiling ? '编译中…' : '编译 PDF'}</button>
        <button className="button" disabled={compiling || exporting || !project || !pdfURL} onClick={() => void exportPDF()}><Icon name="download"/>{exporting ? '导出中…' : '导出 PDF'}</button>
      </div>
    </header>

    <section className="workspace" ref={workspaceRef} style={{gridTemplateColumns:`${paneSizes[0]}fr 6px ${paneSizes[1]}fr 6px ${paneSizes[2]}fr`}}>
      <aside className="outline panel">
        <div className="panel-title"><span>简历结构</span><b>{project?.blocks.length || 0}</b></div>
        <div className="file-row"><Icon name="file"/><div><strong>{project?.mainFile || '未打开项目'}</strong><small>{project?.path || '选择一个包含 .tex 的目录'}</small></div></div>
        <nav className="block-list">
          {(project?.blocks || []).map((block, index) => <button key={block.id} className={activeID === block.id && mode === 'focus' ? 'active' : ''} onClick={() => void selectBlock(block)}>
            <span className="block-index">{String(index+1).padStart(2,'0')}</span><span><strong>{block.title}</strong><small>{typeLabels[block.type] || block.type} · L{block.startLine}–{block.endLine}</small></span>
          </button>)}
        </nav>
        <div className="marker-tip"><code>% @resume-block …</code><p>用注释标记段落，原始 LaTeX 可继续独立编译。</p></div>
      </aside>

      <div className="pane-splitter" role="separator" aria-orientation="vertical" aria-label="调整结构树和编辑器宽度" aria-valuenow={Math.round(paneSizes[0] * 100)} tabIndex={0} title="拖动调整宽度，双击恢复默认布局" onPointerDown={event => beginResize(0, event)} onKeyDown={event => nudgeSplitter(0, event)} onDoubleClick={() => setPaneSizes([...defaultPaneSizes])}><span/></div>

      <section className="editor panel">
        <div className="editor-head">
          <div><small>{mode === 'focus' ? typeLabels[active?.type || 'section'] : '完整源文件'}</small><h1>{mode === 'focus' ? active?.title || '选择段落' : project?.mainFile}</h1></div>
          <div className="segmented"><button className={mode==='focus'?'active':''} onClick={() => void switchMode('focus')}><Icon name="focus"/>聚焦</button><button className={mode==='source'?'active':''} onClick={() => void switchMode('source')}><Icon name="code"/>源码</button></div>
        </div>
        <div className="editor-meta"><span>{mode === 'focus' ? active?.id : project?.path}</span><span>{draft.split('\n').length} 行 · {draft.length} 字符</span></div>
        <textarea className="code-editor" spellCheck={false} value={draft} onChange={e => updateDraft(e.target.value)} disabled={!project || (mode==='focus' && !active)} aria-label="LaTeX 编辑器"/>
        <div className="editor-foot"><span className={dirty?'dirty':'saved'}>{dirty ? '● 未保存更改' : '✓ 所有更改已保存'}</span><span>自动保存 800ms</span></div>
      </section>

      <div className="pane-splitter" role="separator" aria-orientation="vertical" aria-label="调整编辑器和 PDF 预览宽度" aria-valuenow={Math.round((paneSizes[0] + paneSizes[1]) * 100)} tabIndex={0} title="拖动调整宽度，双击恢复默认布局" onPointerDown={event => beginResize(1, event)} onKeyDown={event => nudgeSplitter(1, event)} onDoubleClick={() => setPaneSizes([...defaultPaneSizes])}><span/></div>

      <section className="preview panel">
        <div className="preview-head"><div><Icon name="pdf"/><span>PDF 预览</span></div><span className={`compile-badge ${compileResult?.success?'ok':compileResult?'error':''}`}>{compiling?'构建中':compileResult?.success?'最新':compileResult?'有错误':'等待'}</span></div>
        <div className="paper-stage">
          {pdfURL ? <PDFPreview url={pdfURL} focus={pdfFocus}/> : <div className="empty-preview"><div className="page-skeleton"><span/><span/><span/><span/></div><h3>等待第一版 PDF</h3><p>{compileResult?.diagnostics[0]?.message || '保存后会自动调用本地 LaTeX 编译器'}</p></div>}
        </div>
        {!!compileResult?.diagnostics.length && <div className="diagnostics">
          <div className="diagnostic-title"><Icon name="alert"/><strong>编译诊断</strong><span>{compileResult.diagnostics.length}</span></div>
          {compileResult.diagnostics.slice(0,3).map((d,i)=><div className="diagnostic" key={i}><code>{d.file || project?.mainFile}{d.line ? `:${d.line}`:''}</code><p>{d.message}</p></div>)}
        </div>}
      </section>
    </section>
  </main>
}

export default App

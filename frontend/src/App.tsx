import { useEffect, useMemo, useRef, useState } from 'react'
import './App.css'
import { Compile, CreateDemoProject, OpenProject, SaveBlock, SaveSource } from '../bindings/resumestudio/app'

type Block = { id:string; type:string; title:string; file:string; startLine:number; endLine:number; content:string }
type Project = { path:string; name:string; mainFile:string; source:string; blocks:Block[] }
type Diagnostic = { severity:string; message:string; file?:string; line?:number }
type CompileResult = { revision:number; success:boolean; stale:boolean; engine:string; durationMs:number; pdfBase64?:string; diagnostics:Diagnostic[] }
type Mode = 'focus' | 'source'

function normalizeProject(next: Project): Project { return { ...next, blocks: next.blocks || [] } }

const typeLabels: Record<string, string> = { experience:'工作经历', project:'项目经历', education:'教育经历', skills:'技能', summary:'个人简介', section:'其他' }

function Icon({ name }: { name:'file'|'folder'|'play'|'save'|'code'|'focus'|'pdf'|'check'|'alert' }) {
  const paths: Record<string, JSX.Element> = {
    file:<><path d="M14 2H6a2 2 0 0 0-2 2v16h16V8z"/><path d="M14 2v6h6"/></>,
    folder:<><path d="M3 7h6l2 2h10v10H3z"/><path d="M3 7V5h6l2 2"/></>,
    play:<path d="m8 5 11 7-11 7z"/>, save:<><path d="M5 3h12l2 2v16H5z"/><path d="M8 3v6h8V3M8 17h8"/></>,
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
  const [compileResult, setCompileResult] = useState<CompileResult | null>(null)
  const [pdfURL, setPdfURL] = useState('')
  const [message, setMessage] = useState('正在准备示例项目…')
  const saveTimer = useRef<number>()
  const requestRef = useRef(0)

  const active = useMemo(() => project?.blocks.find(b => b.id === activeID), [project, activeID])

  useEffect(() => { CreateDemoProject().then(next => loadState(next as Project)).catch(showError) }, [])
  useEffect(() => () => { if (pdfURL) URL.revokeObjectURL(pdfURL) }, [pdfURL])

  function loadState(next: Project, preferredID?: string) {
    const normalized = normalizeProject(next)
	setProject(normalized)
	const id = preferredID && normalized.blocks.some(b => b.id === preferredID) ? preferredID : normalized.blocks[0]?.id || ''
	setActiveID(id)
	const value = mode === 'source' ? normalized.source : normalized.blocks.find(b => b.id === id)?.content || ''
	setDraft(value); setSavedDraft(value)
	setMessage(`${normalized.name} · ${normalized.blocks.length} 个结构化段落`)
	void runCompile(normalized.path)
  }

  async function selectBlock(block: Block) {
    const next = draft !== savedDraft ? await saveNow() : project
    const fresh = next?.blocks.find(b => b.id === block.id) || block
    setMode('focus'); setActiveID(fresh.id); setDraft(fresh.content); setSavedDraft(fresh.content)
  }

  async function switchMode(nextMode: Mode) {
    if (!project || nextMode === mode) return
    const next = draft !== savedDraft ? await saveNow() : project
    if (!next) return
    setMode(nextMode)
    const value = nextMode === 'source' ? next.source : next.blocks.find(b => b.id === activeID)?.content || ''
    setDraft(value); setSavedDraft(value)
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
      await runCompile(next.path)
      return next
    } catch (error) { showError(error) }
  }

  async function runCompile(path = project?.path) {
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
      setMessage(result.success ? `编译完成 · ${result.engine} · ${result.durationMs}ms` : '编译失败 · 已保留上一次成功预览')
    } catch (error) { showError(error) }
    finally { if (request === requestRef.current) setCompiling(false) }
  }

  async function chooseProject() {
    try { const next = await OpenProject() as Project; if (next?.path) loadState(next) } catch (error) { showError(error) }
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
      </div>
    </header>

    <section className="workspace">
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

      <section className="editor panel">
        <div className="editor-head">
          <div><small>{mode === 'focus' ? typeLabels[active?.type || 'section'] : '完整源文件'}</small><h1>{mode === 'focus' ? active?.title || '选择段落' : project?.mainFile}</h1></div>
          <div className="segmented"><button className={mode==='focus'?'active':''} onClick={() => void switchMode('focus')}><Icon name="focus"/>聚焦</button><button className={mode==='source'?'active':''} onClick={() => void switchMode('source')}><Icon name="code"/>源码</button></div>
        </div>
        <div className="editor-meta"><span>{mode === 'focus' ? active?.id : project?.path}</span><span>{draft.split('\n').length} 行 · {draft.length} 字符</span></div>
        <textarea className="code-editor" spellCheck={false} value={draft} onChange={e => updateDraft(e.target.value)} disabled={!project || (mode==='focus' && !active)} aria-label="LaTeX 编辑器"/>
        <div className="editor-foot"><span className={dirty?'dirty':'saved'}>{dirty ? '● 未保存更改' : '✓ 所有更改已保存'}</span><span>自动保存 800ms</span></div>
      </section>

      <section className="preview panel">
        <div className="preview-head"><div><Icon name="pdf"/><span>PDF 预览</span></div><span className={`compile-badge ${compileResult?.success?'ok':compileResult?'error':''}`}>{compiling?'构建中':compileResult?.success?'最新':compileResult?'有错误':'等待'}</span></div>
        <div className="paper-stage">
          {pdfURL ? <iframe src={`${pdfURL}#toolbar=0&navpanes=0&view=FitH`} title="PDF 预览"/> : <div className="empty-preview"><div className="page-skeleton"><span/><span/><span/><span/></div><h3>等待第一版 PDF</h3><p>{compileResult?.diagnostics[0]?.message || '保存后会自动调用本地 LaTeX 编译器'}</p></div>}
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

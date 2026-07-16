import { useEffect, useMemo, useRef, useState, type WheelEvent as ReactWheelEvent } from 'react'
import { GlobalWorkerOptions, getDocument } from 'pdfjs-dist'
import pdfWorkerURL from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

GlobalWorkerOptions.workerSrc = pdfWorkerURL

export type PDFLocation = {
  page:number
  left:number
  top:number
  width:number
  height:number
}

type Props = {
  url:string
  focus:PDFLocation | null
}

const clampZoom = (value: number) => Math.min(250, Math.max(50, Math.round(value)))

export default function PDFPreview({ url, focus }: Props) {
  const viewerRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState('')
  const [manualZoom, setManualZoom] = useState<number | null>(null)
  const [displayZoom, setDisplayZoom] = useState(100)
  const [fitRevision, setFitRevision] = useState(0)
  const focusKey = useMemo(() => focus
    ? `${focus.page}:${focus.left}:${focus.top}:${focus.width}:${focus.height}`
    : 'page', [focus])

  useEffect(() => setManualZoom(null), [url, focusKey])

  useEffect(() => {
    const viewer = viewerRef.current
    if (!viewer || !url) return
    const canvasViewer: HTMLDivElement = viewer
    let cancelled = false
    const loadingTask = getDocument(url)
    canvasViewer.replaceChildren()
    setError('')

    async function render() {
      try {
        const pdfDocument = await loadingTask.promise
        if (cancelled) return
        const targetPage = await pdfDocument.getPage(focus?.page || 1)
        const natural = targetPage.getViewport({ scale: 1 })
        const pageFitScale = Math.min(1.25, Math.max(.5, (canvasViewer.clientWidth - 48) / natural.width))
        const focusWidthScale = focus ? (canvasViewer.clientWidth - 88) / Math.max(80, focus.width + 32) : pageFitScale
        const focusHeightScale = focus ? (canvasViewer.clientHeight - 108) / Math.max(80, focus.height + 72) : pageFitScale
        const autoScale = focus
          ? Math.max(pageFitScale, Math.min(1.7, focusWidthScale, focusHeightScale))
          : pageFitScale
        const zoom = clampZoom(manualZoom ?? autoScale * 100)
        const scale = zoom / 100
        setDisplayZoom(zoom)

        const pages = window.document.createElement('div')
        pages.className = 'pdf-pages'
        canvasViewer.appendChild(pages)

        for (let pageNumber = 1; pageNumber <= pdfDocument.numPages; pageNumber++) {
          const page = pageNumber === (focus?.page || 1) ? targetPage : await pdfDocument.getPage(pageNumber)
          if (cancelled) return
          const viewport = page.getViewport({ scale })
          const pageElement = window.document.createElement('div')
          const canvas = window.document.createElement('canvas')
          const context = canvas.getContext('2d')
          if (!context) throw new Error('无法创建 PDF 画布')

          pageElement.className = 'pdf-page'
          pageElement.style.width = `${viewport.width}px`
          pageElement.style.height = `${viewport.height}px`
          canvas.style.width = `${viewport.width}px`
          canvas.style.height = `${viewport.height}px`
          const ratio = window.devicePixelRatio || 1
          canvas.width = Math.floor(viewport.width * ratio)
          canvas.height = Math.floor(viewport.height * ratio)
          pageElement.appendChild(canvas)
          pages.appendChild(pageElement)

          if (focus?.page === pageNumber) {
            const highlight = window.document.createElement('div')
            highlight.className = 'pdf-focus-highlight'
            const highlightLeft = Math.max(0, focus.left * scale)
            highlight.style.left = `${highlightLeft}px`
            highlight.style.top = `${Math.max(0, focus.top * scale)}px`
            highlight.style.width = `${Math.min(viewport.width - highlightLeft, Math.max(48, focus.width * scale))}px`
            highlight.style.height = `${Math.max(28, focus.height * scale)}px`
            highlight.setAttribute('aria-label', '当前聚焦段落')
            pageElement.appendChild(highlight)
          }

          await page.render({
            canvas,
            canvasContext: context,
            viewport,
            transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0],
          }).promise

          if (focus?.page === pageNumber && !cancelled) {
            requestAnimationFrame(() => canvasViewer.scrollTo({
              left: Math.max(0, pageElement.offsetLeft + (focus.left + focus.width / 2) * scale - canvasViewer.clientWidth / 2),
              top: Math.max(0, pageElement.offsetTop + (focus.top + focus.height / 2) * scale - canvasViewer.clientHeight / 2),
              behavior: 'smooth',
            }))
          }
        }
      } catch (reason) {
        if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason))
      }
    }

    void render()
    return () => {
      cancelled = true
      void loadingTask.destroy()
    }
  }, [url, focus, manualZoom, fitRevision])

  function changeZoom(delta: number) {
    setManualZoom(clampZoom((manualZoom ?? displayZoom) + delta))
  }

  function fitContent() {
    setManualZoom(null)
    setFitRevision(value => value + 1)
  }

  function handleWheel(event: ReactWheelEvent<HTMLDivElement>) {
    if (!event.ctrlKey) return
    event.preventDefault()
    changeZoom(event.deltaY < 0 ? 10 : -10)
  }

  return <div className="pdf-preview-shell" onWheel={handleWheel}>
    <div className="pdf-zoom-toolbar" aria-label="PDF 缩放控制">
      <button disabled={!url || displayZoom <= 50} onClick={() => changeZoom(-10)} title="缩小">−</button>
      <button className="zoom-value" disabled={!url} onClick={() => setManualZoom(100)} title="恢复 100%">{displayZoom}%</button>
      <button disabled={!url || displayZoom >= 250} onClick={() => changeZoom(10)} title="放大">＋</button>
      <span/>
      <button className="fit-button" disabled={!url} onClick={fitContent} title={focus ? '完整显示当前文字块' : '适合页面宽度'}>适应</button>
    </div>
    <div className="pdf-canvas-viewer" ref={viewerRef}/>
    {error && <div className="pdf-render-error">PDF 渲染失败：{error}</div>}
  </div>
}

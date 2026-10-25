import { useEffect, useRef, useState } from 'react'

type Detector = { detect: (source: CanvasImageSource) => Promise<Array<{ rawValue: string }>> }
type DetectorConstructor = new (options: { formats: string[] }) => Detector
type ScannerState = 'starting' | 'running' | 'denied' | 'unsupported'

const SCAN_EVERY_MS = 120
const MAX_SIDE = 640

type Decode = () => Promise<string | null>

/** iPhone y otros navegadores sin BarcodeDetector: jsQR, cargado solo cuando hace falta. */
async function createJsQrDecoder(video: HTMLVideoElement): Promise<Decode> {
  const { default: jsQR } = await import('jsqr')
  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d', { willReadFrequently: true })
  return async () => {
    if (!context || !video.videoWidth) return null
    // Se reduce el cuadro: decodificar a 640 px es mucho más rápido y alcanza para un QR de este tamaño.
    const scale = Math.min(1, MAX_SIDE / Math.max(video.videoWidth, video.videoHeight))
    canvas.width = Math.round(video.videoWidth * scale)
    canvas.height = Math.round(video.videoHeight * scale)
    context.drawImage(video, 0, 0, canvas.width, canvas.height)
    const frame = context.getImageData(0, 0, canvas.width, canvas.height)
    return jsQR(frame.data, canvas.width, canvas.height)?.data ?? null
  }
}

const MESSAGES: Record<Exclude<ScannerState, 'running'>, string> = {
  starting: 'Abriendo la cámara…',
  denied: 'No hay permiso para usar la cámara. Concédelo en los ajustes del navegador y vuelve a intentarlo.',
  unsupported: 'Este navegador no puede usar la cámara. Abre sIsA en Safari o Chrome, con conexión segura (https).',
}

type ScannerProps = {
  /** Cada QR leído, sin filtrar: quien llama decide cuándo ignorar repeticiones. */
  onCode: (text: string) => void
}

export function Scanner({ onCode }: ScannerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const callback = useRef(onCode)
  const [state, setState] = useState<ScannerState>('starting')

  useEffect(() => {
    callback.current = onCode
  }, [onCode])

  useEffect(() => {
    let stopped = false
    let stream: MediaStream | null = null
    let timer: ReturnType<typeof setTimeout> | undefined

    async function start() {
      if (!navigator.mediaDevices?.getUserMedia) {
        setState('unsupported')
        return
      }
      try {
        stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' }, audio: false })
      } catch {
        if (!stopped) setState('denied')
        return
      }
      const video = videoRef.current
      if (stopped || !video) {
        stream.getTracks().forEach((track) => track.stop())
        return
      }
      video.srcObject = stream
      await video.play().catch(() => undefined)
      if (stopped) return
      setState('running')

      const Native = (window as unknown as { BarcodeDetector?: DetectorConstructor }).BarcodeDetector
      let decode: Decode
      if (Native) {
        const detector = new Native({ formats: ['qr_code'] })
        decode = async () => (await detector.detect(video))[0]?.rawValue ?? null
      } else {
        decode = await createJsQrDecoder(video)
      }

      const loop = async () => {
        if (stopped) return
        try {
          const text = await decode()
          if (text) callback.current(text)
        } catch {
          // Cuadro ilegible: se intenta con el siguiente.
        }
        timer = setTimeout(() => void loop(), SCAN_EVERY_MS)
      }
      void loop()
    }

    void start()
    return () => {
      stopped = true
      clearTimeout(timer)
      stream?.getTracks().forEach((track) => track.stop())
    }
  }, [])

  return (
    <div className="flex flex-col gap-3">
      <video
        ref={videoRef}
        playsInline
        muted
        aria-label="Cámara para leer el QR"
        className={`aspect-[4/3] w-full rounded-2xl bg-neutral-900 object-cover ${state === 'running' ? '' : 'hidden'}`}
      />
      {state !== 'running' && (
        <p role={state === 'starting' ? 'status' : 'alert'} className="text-lg text-neutral-700">
          {MESSAGES[state]}
        </p>
      )}
    </div>
  )
}

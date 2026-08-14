<template>
  <div ref="host" class="particle-backdrop" aria-hidden="true">
    <canvas ref="canvas" />
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

const host = ref(null)
const canvas = ref(null)
let frame = 0
let resizeObserver
let particles = []
let cleanup = () => {}
const pointer = { x: -1000, y: -1000, active: false }

function createParticles(width, height, count) {
  particles = Array.from({ length: count }, () => ({
    x: Math.random() * width,
    y: Math.random() * height,
    vx: (Math.random() - 0.5) * 0.22,
    vy: (Math.random() - 0.5) * 0.22
  }))
}

onMounted(() => {
  const surface = host.value
  const target = canvas.value
  if (!surface || !target) return

  const context = target.getContext('2d')
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const liteMode = navigator.hardwareConcurrency > 0 && navigator.hardwareConcurrency < 4
  const particleCount = reduceMotion ? 28 : liteMode ? 36 : 60
  let width = 0
  let height = 0

  const resize = () => {
    const rect = surface.getBoundingClientRect()
    width = Math.max(1, rect.width)
    height = Math.max(1, rect.height)
    const ratio = Math.min(window.devicePixelRatio || 1, 1.5)
    target.width = Math.round(width * ratio)
    target.height = Math.round(height * ratio)
    target.style.width = `${width}px`
    target.style.height = `${height}px`
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
    createParticles(width, height, particleCount)
  }

  const movePointer = (event) => {
    const rect = surface.getBoundingClientRect()
    pointer.x = event.clientX - rect.left
    pointer.y = event.clientY - rect.top
    pointer.active = pointer.x >= 0 && pointer.x <= width && pointer.y >= 0 && pointer.y <= height
  }
  const leavePointer = () => { pointer.active = false }

  const draw = () => {
    context.clearRect(0, 0, width, height)

    particles.forEach((particle) => {
      if (!reduceMotion) {
        if (pointer.active) {
          const dx = particle.x - pointer.x
          const dy = particle.y - pointer.y
          const distance = Math.hypot(dx, dy)
          if (distance > 0 && distance < 165) {
            const force = (165 - distance) / 165
            particle.x += (dx / distance) * force * 1.55
            particle.y += (dy / distance) * force * 1.55
          }
        }
        particle.x += particle.vx
        particle.y += particle.vy
        if (particle.x < 0) particle.x = width
        if (particle.x > width) particle.x = 0
        if (particle.y < 0) particle.y = height
        if (particle.y > height) particle.y = 0
      }
    })

    particles.forEach((particle, index) => {
      context.fillStyle = index % 5 === 0 ? 'rgba(165, 243, 252, .82)' : 'rgba(103, 232, 249, .66)'
      context.beginPath()
      context.arc(particle.x, particle.y, index % 5 === 0 ? 1.5 : 1.25, 0, Math.PI * 2)
      context.fill()

      for (let next = index + 1; next < particles.length; next += 1) {
        const other = particles[next]
        const distance = Math.hypot(particle.x - other.x, particle.y - other.y)
        if (distance >= 170) continue
        context.strokeStyle = `rgba(56, 189, 248, ${(1 - distance / 170) * 0.3})`
        context.lineWidth = 0.8
        context.beginPath()
        context.moveTo(particle.x, particle.y)
        context.lineTo(other.x, other.y)
        context.stroke()
      }
    })

    if (!reduceMotion) frame = requestAnimationFrame(draw)
  }

  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(surface)
  window.addEventListener('pointermove', movePointer, { passive: true })
  window.addEventListener('pointerleave', leavePointer)
  resize()
  draw()

  cleanup = () => {
    cancelAnimationFrame(frame)
    resizeObserver?.disconnect()
    window.removeEventListener('pointermove', movePointer)
    window.removeEventListener('pointerleave', leavePointer)
  }
})

onBeforeUnmount(() => cleanup())
</script>

<style scoped>
.particle-backdrop {
  position: absolute;
  z-index: 0;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
    background:
    radial-gradient(circle at 52% 10%, rgba(255,255,255,.095), transparent 35%),
    radial-gradient(circle at 70% 72%, rgba(56,189,248,.12), transparent 48%),
    rgba(8, 13, 22, .28);
}
canvas { display: block; opacity: .96; filter: drop-shadow(0 0 5px rgba(56,189,248,.22)); }
</style>

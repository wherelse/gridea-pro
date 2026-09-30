<template>
  <div class="h-full flex flex-col p-6 overflow-hidden" @dragover.prevent @drop.prevent="onDrop">
    <header class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <PhotoIcon class="w-6 h-6" />
        <h1 class="text-lg font-semibold">{{ t('gallery.title') }}</h1>
        <span class="text-sm text-muted-foreground">{{ t('gallery.count', { n: galleryStore.total }) }}</span>
      </div>
      <Button class="gap-2" @click="pickFiles">
        <ArrowUpTrayIcon class="w-4 h-4" />
        {{ t('gallery.upload') }}
      </Button>
    </header>

    <p class="text-xs text-muted-foreground mb-4">{{ t('gallery.hint') }}</p>

    <div v-if="galleryStore.total === 0"
      class="flex-1 flex flex-col items-center justify-center text-muted-foreground gap-3">
      <PhotoIcon class="w-16 h-16 opacity-40" />
      <div>{{ t('gallery.empty') }}</div>
    </div>

    <div v-else class="flex-1 overflow-auto">
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
        <div v-for="(photo, index) in galleryStore.photos" :key="photo.id"
          class="group relative rounded-lg border bg-card overflow-hidden" draggable="true"
          @dragstart="onItemDragStart(index)" @dragover.prevent @drop.stop.prevent="onItemDrop(index)"
          @dragend="dragIndex = -1">
          <div class="relative aspect-square">
            <img :src="imgUrl(photo.image)" class="w-full h-full object-cover" loading="lazy" />
            <div v-if="photo.isTop"
              class="absolute top-2 left-2 flex items-center gap-1 px-2 py-0.5 rounded-full bg-amber-500 text-white text-xs">
              <StarIcon class="w-3 h-3" /> {{ t('gallery.cover') }}
            </div>
            <div
              class="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
              <Button v-if="!photo.isTop" size="icon" variant="secondary" :title="t('gallery.setCover')"
                @click="galleryStore.setCover(photo.id)">
                <StarIcon class="w-4 h-4" />
              </Button>
              <Button size="icon" variant="destructive" :title="t('gallery.delete')" @click="removePhoto(photo.id)">
                <TrashIcon class="w-4 h-4" />
              </Button>
              <span class="cursor-move text-white/90" :title="t('gallery.dragHint')"><Bars3Icon class="w-4 h-4" /></span>
            </div>
          </div>
          <div class="p-2">
            <Input v-model="photo.description" :placeholder="t('gallery.descPlaceholder')"
              @blur="galleryStore.updatePhoto(photo)" @keyup.enter="galleryStore.updatePhoto(photo)" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useGalleryStore } from '@/stores/gallery'
import { useSiteStore } from '@/stores/site'
import { useImageUrl } from '@/composables/useImageUrl'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PhotoIcon, ArrowUpTrayIcon, StarIcon, TrashIcon, Bars3Icon } from '@heroicons/vue/24/outline'

const { t } = useI18n()
const galleryStore = useGalleryStore()
const siteStore = useSiteStore()
const { getImageUrl } = useImageUrl()

const imgUrl = (p: string): string => {
  if (!p) return ''
  if (p.startsWith('http') || p.startsWith('data:')) return p
  if (p.startsWith('/images/')) return getImageUrl(`${siteStore.site.appDir}${p}`)
  return getImageUrl(p)
}

async function pickFiles() {
  const path = await (window as any).go.app.App.OpenImageDialog()
  if (!path) return
  try {
    await galleryStore.addByFile(path)
  } catch (e) {
    console.error('[gallery] add image failed', e)
  }
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const res = String(reader.result || '')
      const comma = res.indexOf(',')
      resolve(comma >= 0 ? res.slice(comma + 1) : res)
    }
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function addFiles(files: FileList | File[]) {
  const arr = Array.from(files).filter((f) => f.type.startsWith('image/'))
  for (const f of arr) {
    try {
      const b64 = await fileToBase64(f)
      await galleryStore.addByBytes(f.name, b64)
    } catch (e) {
      console.error('[gallery] add image failed', e)
    }
  }
}

function onDrop(e: DragEvent) {
  if (e.dataTransfer?.files?.length) addFiles(e.dataTransfer.files)
}

function onPaste(e: ClipboardEvent) {
  const files = e.clipboardData?.files
  if (files && files.length) addFiles(files)
}

function removePhoto(id: string) {
  if (!confirm(t('gallery.deleteConfirm'))) return
  galleryStore.removePhoto(id)
}

// 拖动排序
let dragIndex = -1
function onItemDragStart(index: number) {
  dragIndex = index
}
async function onItemDrop(index: number) {
  if (dragIndex < 0 || dragIndex === index) return
  const arr = galleryStore.photos.slice()
  const [moved] = arr.splice(dragIndex, 1)
  arr.splice(index, 0, moved)
  dragIndex = -1
  await galleryStore.reorder(arr.map((p) => p.id))
}

onMounted(() => {
  galleryStore.fetchPhotos()
  document.addEventListener('paste', onPaste)
})
onUnmounted(() => {
  document.removeEventListener('paste', onPaste)
})
</script>

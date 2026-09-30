import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { domain } from '@/wailsjs/go/models'
import {
  LoadGalleryFromFrontend,
  AddPhotoByFileFromFrontend,
  AddPhotoByBytesFromFrontend,
  UpdatePhotoFromFrontend,
  DeletePhotoFromFrontend,
  ReorderPhotosFromFrontend,
  SetCoverFromFrontend,
} from '@/wailsjs/go/facade/GalleryFacade'
import { safeEventsEmit } from '@/helpers/wailsRuntime'

export const useGalleryStore = defineStore('gallery', () => {
  const photos = ref<domain.GalleryPhoto[]>([])
  const loading = ref(false)

  const total = computed(() => photos.value.length)
  const cover = computed(() => photos.value.find((p) => p.isTop) || photos.value[0] || null)

  // 相册数据变化会影响站点渲染，通知后端重新渲染
  const triggerReload = () => safeEventsEmit('app-site-reload')

  function apply(list: domain.GalleryPhoto[] | null) {
    photos.value = (list || []).map((p) => new domain.GalleryPhoto(p))
  }

  async function fetchPhotos() {
    loading.value = true
    try {
      apply(await LoadGalleryFromFrontend())
    } finally {
      loading.value = false
    }
  }

  async function addByFile(sourcePath: string, description = '') {
    apply(await AddPhotoByFileFromFrontend(sourcePath, description))
    triggerReload()
  }

  async function addByBytes(name: string, dataBase64: string, description = '') {
    apply(await AddPhotoByBytesFromFrontend(name, dataBase64, description))
    triggerReload()
  }

  async function updatePhoto(photo: domain.GalleryPhoto) {
    apply(await UpdatePhotoFromFrontend(new domain.GalleryPhoto(photo)))
    triggerReload()
  }

  async function removePhoto(id: string) {
    apply(await DeletePhotoFromFrontend(id))
    triggerReload()
  }

  async function reorder(ids: string[]) {
    apply(await ReorderPhotosFromFrontend(ids))
    triggerReload()
  }

  async function setCover(id: string) {
    apply(await SetCoverFromFrontend(id))
    triggerReload()
  }

  return {
    photos,
    loading,
    total,
    cover,
    fetchPhotos,
    addByFile,
    addByBytes,
    updatePhoto,
    removePhoto,
    reorder,
    setCover,
  }
})

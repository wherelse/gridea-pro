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
  SetSortFromFrontend,
} from '@/wailsjs/go/facade/GalleryFacade'
import { safeEventsEmit } from '@/helpers/wailsRuntime'

export const useGalleryStore = defineStore('gallery', () => {
  const photos = ref<domain.GalleryPhoto[]>([])
  const sort = ref<string>('manual')
  const loading = ref(false)

  const total = computed(() => photos.value.length)
  const cover = computed(() => photos.value.find((p) => p.isTop) || photos.value[0] || null)

  // 相册数据变化会影响站点渲染，通知后端重新渲染
  const triggerReload = () => safeEventsEmit('app-site-reload')

  function applyList(list: domain.GalleryPhoto[] | null) {
    photos.value = (list || []).map((p) => new domain.GalleryPhoto(p))
  }
  function applyData(data: domain.GalleryData | null) {
    photos.value = (data?.photos || []).map((p) => new domain.GalleryPhoto(p))
    sort.value = data?.sort || 'manual'
  }

  async function fetchPhotos() {
    loading.value = true
    try {
      applyData(await LoadGalleryFromFrontend())
    } finally {
      loading.value = false
    }
  }

  async function addByFile(sourcePath: string) {
    applyList(await AddPhotoByFileFromFrontend(sourcePath))
    triggerReload()
  }

  async function addByBytes(name: string, dataBase64: string) {
    applyList(await AddPhotoByBytesFromFrontend(name, dataBase64))
    triggerReload()
  }

  async function updatePhoto(photo: domain.GalleryPhoto) {
    applyList(await UpdatePhotoFromFrontend(new domain.GalleryPhoto(photo)))
    triggerReload()
  }

  async function removePhoto(id: string) {
    applyList(await DeletePhotoFromFrontend(id))
    triggerReload()
  }

  async function reorder(ids: string[]) {
    applyList(await ReorderPhotosFromFrontend(ids))
    triggerReload()
  }

  async function setCover(id: string) {
    applyList(await SetCoverFromFrontend(id))
    triggerReload()
  }

  async function setSort(value: string) {
    applyData(await SetSortFromFrontend(value))
    triggerReload()
  }

  return {
    photos,
    sort,
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
    setSort,
  }
})

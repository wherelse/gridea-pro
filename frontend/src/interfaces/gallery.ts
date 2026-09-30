export interface IGalleryPhoto {
  id: string
  image: string
  title: string
  location: string
  takenAt: string
  cameraMake: string
  cameraModel: string
  lens: string
  aperture: string
  shutter: string
  iso: string
  focalLength: string
  width: number
  height: number
  isTop: boolean
  createdAt: string
  updatedAt: string
}

export interface IGalleryData {
  sort: string
  photos: IGalleryPhoto[]
}

export const GALLERY_SORTS = ['manual', 'time-desc', 'time-asc'] as const

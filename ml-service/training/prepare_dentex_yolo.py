import json
import random
import shutil
from pathlib import Path


PROJECT_ROOT = Path(__file__).resolve().parents[2]

ANNOTATION_PATH = PROJECT_ROOT / "data/dentex/DENTEX/validation_triple.json"
IMAGES_DIR = PROJECT_ROOT / "data/dentex/extracted/validation_data/quadrant_enumeration_disease/xrays"
OUTPUT_DIR = PROJECT_ROOT / "data/yolo_dentex"

CLASSES = {
    0: "impacted",
    1: "caries",
    2: "periapical_lesion",
    3: "deep_caries",
}


def yolo_bbox(coco_bbox, image_width, image_height):
    x, y, width, height = coco_bbox

    x_center = (x + width / 2) / image_width
    y_center = (y + height / 2) / image_height
    norm_width = width / image_width
    norm_height = height / image_height

    return x_center, y_center, norm_width, norm_height


def clean_output_dir():
    if OUTPUT_DIR.exists():
        shutil.rmtree(OUTPUT_DIR)

    for split in ["train", "val"]:
        (OUTPUT_DIR / "images" / split).mkdir(parents=True, exist_ok=True)
        (OUTPUT_DIR / "labels" / split).mkdir(parents=True, exist_ok=True)


def main():
    if not ANNOTATION_PATH.exists():
        raise FileNotFoundError(f"Annotation file not found: {ANNOTATION_PATH}")

    if not IMAGES_DIR.exists():
        raise FileNotFoundError(f"Images dir not found: {IMAGES_DIR}")

    clean_output_dir()

    with ANNOTATION_PATH.open("r", encoding="utf-8") as file:
        data = json.load(file)

    images = {image["id"]: image for image in data["images"]}

    annotations_by_image = {}
    for annotation in data["annotations"]:
        image_id = annotation["image_id"]
        annotations_by_image.setdefault(image_id, []).append(annotation)

    image_ids = list(images.keys())
    random.seed(42)
    random.shuffle(image_ids)

    split_index = int(len(image_ids) * 0.8)
    train_ids = set(image_ids[:split_index])

    for image_id in image_ids:
        image = images[image_id]
        file_name = image["file_name"]
        width = image["width"]
        height = image["height"]

        split = "train" if image_id in train_ids else "val"

        source_image = IMAGES_DIR / file_name
        if not source_image.exists():
            print(f"Skip missing image: {source_image}")
            continue

        target_image = OUTPUT_DIR / "images" / split / file_name
        target_label = OUTPUT_DIR / "labels" / split / f"{Path(file_name).stem}.txt"

        shutil.copy2(source_image, target_image)

        label_lines = []

        for annotation in annotations_by_image.get(image_id, []):
            class_id = int(annotation["category_id_3"])

            if class_id not in CLASSES:
                continue

            bbox = yolo_bbox(annotation["bbox"], width, height)

            label_lines.append(
                f"{class_id} " + " ".join(f"{value:.6f}" for value in bbox)
            )

        target_label.write_text("\n".join(label_lines), encoding="utf-8")

    data_yaml = OUTPUT_DIR / "data.yaml"
    data_yaml.write_text(
        "\n".join(
            [
                f"path: {OUTPUT_DIR}",
                "train: images/train",
                "val: images/val",
                "",
                f"nc: {len(CLASSES)}",
                "names:",
                "  0: impacted",
                "  1: caries",
                "  2: periapical_lesion",
                "  3: deep_caries",
                "",
            ]
        ),
        encoding="utf-8",
    )

    print("YOLO dataset prepared:")
    print(f"Output: {OUTPUT_DIR}")
    print(f"Train images: {len(list((OUTPUT_DIR / 'images/train').glob('*.png')))}")
    print(f"Val images: {len(list((OUTPUT_DIR / 'images/val').glob('*.png')))}")


if __name__ == "__main__":
    main()
import HeroSection from "../components/sections/HeroSection"
import FeaturesShowcase from "../components/sections/FeaturesShowcase"
import type { ShowcaseSlide } from "../components/sections/FeaturesShowcase"

import img1 from "../assets/images/1.png"
import img2 from "../assets/images/2.jpg"
import img3 from "../assets/images/3.jpg"
import img4 from "../assets/images/4.jpg"

const slides: ShowcaseSlide[] = [
    {
        title: 'Автоматически шифрует файлы на стороне отправителя.',
        text: 'Описание функции',
        image: img1
    },
    {
        title: 'Гибкие политики доступа',
        text: 'Описание функции',
        image: img2
    },
    {
        title: 'Журнал аудита действий',
        text: 'Описание функции',
        image: img3
    },
    {
        title: 'Удаление данных',
        text: 'Описание функции',
        image: img4
    },
]

export default function HomePage() {
    return (
        <>
            <HeroSection/>
            <FeaturesShowcase slides={slides}/>
        </>
    )
}
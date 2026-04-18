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
        text: 'Все файлы шифруются перед отправкой прямо в вашем браузере. Даже администраторы системы не имеют доступа к содержимому — только получатель с нужным ключом может расшифровать данные.',
        image: img1
    },
    {
        title: 'Гибкие политики доступа',
        text: 'Устанавливайте срок действия ссылки, ограничивайте количество скачиваний и задавайте список разрешённых получателей. Вы полностью контролируете, кто и как долго может получить доступ к вашим файлам.',
        image: img2
    },
    {
        title: 'Журнал аудита действий',
        text: 'Каждое действие в системе — загрузка, скачивание, отзыв доступа — фиксируется в защищённом журнале. Администраторы могут отслеживать историю операций и экспортировать отчёты для проверки.',
        image: img3
    },
    {
        title: 'Удаление данных',
        text: 'После истечения срока действия или по запросу отправителя файлы безвозвратно удаляются с серверов. Никаких скрытых копий — данные исчезают полностью и навсегда.',
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